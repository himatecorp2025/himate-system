package main

import (
	"fmt"
	"regexp"
	"strings"
)

var automationContractTokenPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.:-]{2,159}$`)

func validateAutomationManifest(manifest map[string]any) error {
	if manifest == nil {
		return nil
	}
	raw, exists := manifest["automation"]
	if !exists {
		return nil
	}
	a, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("manifest.automation must be an object")
	}
	version := strings.TrimSpace(fmt.Sprint(a["contract_version"]))
	if version != "1" {
		return fmt.Errorf("manifest.automation.contract_version must be 1")
	}
	checkList := func(key string, validate func(string) bool) error {
		rawList, exists := a[key]
		if !exists {
			return nil
		}
		items, ok := rawList.([]any)
		if !ok {
			return fmt.Errorf("manifest.automation.%s must be an array", key)
		}
		seen := map[string]bool{}
		for _, item := range items {
			value := strings.TrimSpace(fmt.Sprint(item))
			if value == "" || !validate(value) || seen[value] {
				return fmt.Errorf("manifest.automation.%s contains an invalid or duplicate value", key)
			}
			seen[value] = true
		}
		return nil
	}
	token := func(v string) bool { return automationContractTokenPattern.MatchString(v) }
	if err := checkList("produces_events", token); err != nil {
		return err
	}
	if err := checkList("consumes_events", token); err != nil {
		return err
	}
	if err := checkList("commands", token); err != nil {
		return err
	}
	if err := checkList("scheduled_actions", token); err != nil {
		return err
	}
	if err := checkList("required_permissions", token); err != nil {
		return err
	}
	if err := checkList("required_modules", func(v string) bool { return moduleKeyPattern.MatchString(v) }); err != nil {
		return err
	}
	return nil
}
