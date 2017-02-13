package timezone

import (
	"reflect"
	"testing"
)

func Test_GetRegionsFromConsoleRegionsOutput(t *testing.T) {
	text := `
Asia/Macau
Asia/Magadan
Europe/Tirane
Europe/Ulyanovsk
Pacific/Norfolk

`

	expected := []string{
		"Asia",
		"Europe",
		"Pacific",
	}

	result := GetRegionsFromConsoleRegionsOutput(text)

	if reflect.DeepEqual(result, expected) == false {
		t.Error("Expected", expected, " got ", result)
	}
}

func Test_GetRegionSubjectsFromConsoleRegionsOutput(t *testing.T) {
	text := `
Asia/Macau
Asia/Magadan
Europe/Tirane
Europe/Ulyanovsk
Pacific/Norfolk

`

	expected := []string{
		"Tirane",
		"Ulyanovsk",
	}

	result := GetRegionSubjectsFromConsoleRegionsOutput(text, "Europe")

	if reflect.DeepEqual(result, expected) == false {
		t.Error("Expected", expected, " got ", result)
	}

}
