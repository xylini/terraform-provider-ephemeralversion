package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// ── ephemeralversion_from_map acceptance tests ───────────────────────────────

func fromMapConfig(secrets map[string]string) string {
	pairs := ""
	for k, v := range secrets {
		pairs += fmt.Sprintf("    %s = %q\n", k, v)
	}
	return fmt.Sprintf(`
resource "ephemeralversion_from_map" "test" {
  values = {
%s  }
}
`, pairs)
}

// TestEphemeralversionFromMap_create verifies that after apply each key in
// versions equals md5 of the corresponding input value.
func TestEphemeralversionFromMap_create(t *testing.T) {
	secrets := map[string]string{
		"db_password": "hunter2",
		"api_key":     "s3cr3t",
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fromMapConfig(secrets),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ephemeralversion_from_map.test",
						tfjsonpath.New("id"),
						knownvalue.StringRegexp(uuidRegexp)),
					statecheck.ExpectKnownValue("ephemeralversion_from_map.test",
						tfjsonpath.New("versions"),
						knownvalue.MapExact(map[string]knownvalue.Check{
							"db_password": knownvalue.StringExact(md5Hex("hunter2")),
							"api_key":     knownvalue.StringExact(md5Hex("s3cr3t")),
						})),
				},
			},
		},
	})
}

// TestEphemeralversionFromMap_idStableOnUpdate verifies UUID id is stable when
// values change, and versions are recalculated.
func TestEphemeralversionFromMap_idStableOnUpdate(t *testing.T) {
	v1 := map[string]string{"key": "value1"}
	v2 := map[string]string{"key": "value2"}

	var firstID string

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fromMapConfig(v1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("ephemeralversion_from_map.test", "id",
						func(v string) error {
							if !uuidRegexp.MatchString(v) {
								return fmt.Errorf("id %q is not a UUID", v)
							}
							firstID = v
							return nil
						}),
					resource.TestCheckResourceAttr("ephemeralversion_from_map.test",
						"versions.key", md5Hex("value1")),
				),
			},
			{
				Config: fromMapConfig(v2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("ephemeralversion_from_map.test", "id",
						func(v string) error {
							if v != firstID {
								return fmt.Errorf("id changed: was %q, now %q", firstID, v)
							}
							return nil
						}),
					resource.TestCheckResourceAttr("ephemeralversion_from_map.test",
						"versions.key", md5Hex("value2")),
				),
			},
		},
	})
}

// TestEphemeralversionFromMap_valuesNotInState verifies that write-only values
// are never present in state after apply.
func TestEphemeralversionFromMap_valuesNotInState(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fromMapConfig(map[string]string{"secret": "topsecret"}),
				Check:  resource.TestCheckNoResourceAttr("ephemeralversion_from_map.test", "values"),
			},
		},
	})
}
