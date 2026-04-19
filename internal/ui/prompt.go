package ui

import (
	"fmt"
	"strings"

	"github.com/manifoldco/promptui"
)

func (u *UI) Prompt(label string, defaultValue string) (string, error) {
	prompt := promptui.Prompt{
		Label:   label,
		Default: defaultValue,
	}
	return prompt.Run()
}

func (u *UI) PromptRequired(label string) (string, error) {
	prompt := promptui.Prompt{
		Label: label,
		Validate: func(s string) error {
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("this field is required")
			}
			return nil
		},
	}
	return prompt.Run()
}

func (u *UI) PromptPassword(label string) (string, error) {
	prompt := promptui.Prompt{
		Label: label,
		Mask:  '*',
	}
	return prompt.Run()
}

func (u *UI) Select(label string, items []string) (int, string, error) {
	prompt := promptui.Select{
		Label: label,
		Items: items,
		Size:  10,
	}
	return prompt.Run()
}

func (u *UI) SelectWithDescriptions(label string, items []SelectItem) (int, string, error) {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "▸ {{ .Name | cyan }} {{ .Description | faint }}",
		Inactive: "  {{ .Name }} {{ .Description | faint }}",
		Selected: "✓ {{ .Name | green }}",
	}

	prompt := promptui.Select{
		Label:     label,
		Items:     items,
		Templates: templates,
		Size:      10,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return 0, "", err
	}
	return idx, items[idx].Name, nil
}

type SelectItem struct {
	Name        string
	Description string
}

func (u *UI) Confirm(label string, defaultYes bool) (bool, error) {
	prompt := promptui.Prompt{
		Label:     label,
		IsConfirm: true,
	}

	if defaultYes {
		prompt.Default = "y"
	} else {
		prompt.Default = "n"
	}

	result, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			return false, nil
		}
		return false, err
	}

	return strings.ToLower(result) == "y" || result == "", nil
}

func (u *UI) MultiSelect(label string, items []string) ([]int, error) {
	var selected []int
	remaining := make([]string, len(items))
	copy(remaining, items)
	originalIndices := make([]int, len(items))
	for i := range originalIndices {
		originalIndices[i] = i
	}

	for {
		displayItems := append([]string{"[Done]"}, remaining...)
		prompt := promptui.Select{
			Label: fmt.Sprintf("%s (selected: %d)", label, len(selected)),
			Items: displayItems,
			Size:  10,
		}

		idx, _, err := prompt.Run()
		if err != nil {
			return selected, err
		}

		if idx == 0 {
			break
		}

		actualIdx := originalIndices[idx-1]
		selected = append(selected, actualIdx)

		remaining = append(remaining[:idx-1], remaining[idx:]...)
		originalIndices = append(originalIndices[:idx-1], originalIndices[idx:]...)

		if len(remaining) == 0 {
			break
		}
	}

	return selected, nil
}
