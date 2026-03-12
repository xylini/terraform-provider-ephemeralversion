package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// protoV6ProviderFactories wires the provider into the test framework.
var protoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"ephemeralversion": providerserver.NewProtocol6WithError(New()),
}

// uuidRegexp matches a standard UUID v4.
var uuidRegexp = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ── md5Hex unit tests ────────────────────────────────────────────────────────

func TestMd5Hex(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "d41d8cd98f00b204e9800998ecf8427e"},
		{"hello", "5d41402abc4b2a76b9719d911017c592"},
		{"terraform", "1b1ed905d54c18e3dd8828986c14be17"},
	}

	for _, tt := range tests {
		got := md5Hex(tt.input)
		if got != tt.expected {
			t.Errorf("md5Hex(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// ── ephemeralversion_from acceptance tests ───────────────────────────────────

func fromConfig(name, value string) string {
	return fmt.Sprintf(`
resource "ephemeralversion_from" "test" {
  name  = %q
  value = %q
}
`, name, value)
}

// TestEphemeralversionFrom_create verifies that after apply:
//   - id is a UUID
//   - version equals md5(value)
//   - name is stored in state
func TestEphemeralversionFrom_create(t *testing.T) {
	const value = "my-secret"
	expectedVersion := md5Hex(value)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fromConfig("my-resource", value),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ephemeralversion_from.test",
						tfjsonpath.New("version"),
						knownvalue.StringExact(expectedVersion)),
					statecheck.ExpectKnownValue("ephemeralversion_from.test",
						tfjsonpath.New("id"),
						knownvalue.StringRegexp(uuidRegexp)),
					statecheck.ExpectKnownValue("ephemeralversion_from.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("my-resource")),
				},
			},
		},
	})
}

// TestEphemeralversionFrom_idStableOnUpdate verifies that the UUID id does not
// change when value is updated, but version is recalculated.
func TestEphemeralversionFrom_idStableOnUpdate(t *testing.T) {
	const value1 = "first-secret"
	const value2 = "second-secret"

	var firstID string

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: create — capture the id.
			{
				Config: fromConfig("my-resource", value1),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ephemeralversion_from.test",
						tfjsonpath.New("version"),
						knownvalue.StringExact(md5Hex(value1))),
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("ephemeralversion_from.test", "id",
						func(v string) error {
							if !uuidRegexp.MatchString(v) {
								return fmt.Errorf("id %q is not a UUID", v)
							}
							firstID = v
							return nil
						}),
				),
			},
			// Step 2: update value — id must be the same UUID, version must change.
			{
				Config: fromConfig("my-resource", value2),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ephemeralversion_from.test",
						tfjsonpath.New("version"),
						knownvalue.StringExact(md5Hex(value2))),
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("ephemeralversion_from.test", "id",
						func(v string) error {
							if v != firstID {
								return fmt.Errorf("id changed: was %q, now %q", firstID, v)
							}
							return nil
						}),
				),
			},
		},
	})
}

// TestEphemeralversionFrom_valueNotInState verifies that the write-only value
// is never present in state after apply.
func TestEphemeralversionFrom_valueNotInState(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fromConfig("my-resource", "super-secret"),
				Check:  resource.TestCheckNoResourceAttr("ephemeralversion_from.test", "value"),
			},
		},
	})
}
