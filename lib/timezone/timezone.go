package timezone

import "strings"

func GetRegionsFromConsoleRegionsOutput(text string) []string {
	regions := make(map[string]bool)

	lines := strings.Split(text, "\n")
	for _, timezone := range lines {
		items := strings.Split(timezone, "/")
		if len(items) > 0 {
			region := items[0]
			if region != "" {
				regions[region] = true
			}
		}
	}

	result := make([]string, 0, len(regions))
	for k := range regions {
		result = append(result, k)
	}

	return result
}

func GetRegionSubjectsFromConsoleRegionsOutput(text, selectedRegion string) []string {
	subjects := make(map[string]bool)

	lines := strings.Split(text, "\n")
	for _, timezone := range lines {
		items := strings.Split(timezone, "/")
		if len(items) > 1 {
			region := items[0]
			subject := items[1]
			if region == selectedRegion {
				subjects[subject] = true
			}
		}
	}

	result := make([]string, 0, len(subjects))
	for k := range subjects {
		result = append(result, k)
	}

	return result
}
