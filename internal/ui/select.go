package ui

import (
	"errors"
	"fmt"
	"strings"

	"opencode-cli/internal/model"

	"github.com/AlecAivazis/survey/v2"
)

func PickProject(projects []model.ProjectRecord, query string) (model.ProjectRecord, error) {
	if query != "" {
		matches := make([]model.ProjectRecord, 0)
		for _, p := range projects {
			if p.ID == query || strings.Contains(strings.ToLower(p.Worktree), strings.ToLower(query)) {
				matches = append(matches, p)
			}
		}
		switch len(matches) {
		case 0:
			return model.ProjectRecord{}, fmt.Errorf("no project matched %q", query)
		case 1:
			return matches[0], nil
		default:
			projects = matches
		}
	}

	labels := make([]string, 0, len(projects))
	lookup := make(map[string]model.ProjectRecord, len(projects))
	for _, p := range projects {
		label := fmt.Sprintf("%s (%s)", p.Worktree, p.ID)
		labels = append(labels, label)
		lookup[label] = p
	}

	choice, err := Select("Select a project:", labels)
	if err != nil {
		return model.ProjectRecord{}, err
	}

	return lookup[choice], nil
}

func PickFiles(files []string, history map[string][]model.FileEvent, snapshots map[string][]model.ContentSnapshot, query string) ([]string, error) {
	if query != "" {
		matches := make([]string, 0)
		for _, f := range files {
			if f == query || strings.Contains(strings.ToLower(f), strings.ToLower(query)) {
				matches = append(matches, f)
			}
		}
		switch len(matches) {
		case 0:
			return nil, fmt.Errorf("no file matched %q", query)
		case 1:
			return matches, nil
		default:
			files = matches
		}
	}

	labels := make([]string, 0, len(files))
	lookup := make(map[string]string, len(files))
	for _, f := range files {
		label := fmt.Sprintf("%s (%d events, %d snapshots)", f, len(history[f]), len(snapshots[f]))
		labels = append(labels, label)
		lookup[label] = f
	}

	if query != "" {
		choice, err := Select("Select a file to reconstruct:", labels)
		if err != nil {
			return nil, err
		}
		return []string{lookup[choice]}, nil
	}

	choices, err := MultiSelect("Select one or more files to reconstruct:", labels)
	if err != nil {
		return nil, err
	}
	if len(choices) == 0 {
		return nil, errors.New("no files selected")
	}

	selected := make([]string, 0, len(choices))
	for _, c := range choices {
		selected = append(selected, lookup[c])
	}

	return selected, nil
}

func Select(message string, options []string) (string, error) {
	if len(options) == 0 {
		return "", errors.New("no options available")
	}
	if len(options) == 1 {
		return options[0], nil
	}

	selected := ""
	prompt := &survey.Select{Message: message, Options: options, PageSize: 20}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return "", err
	}
	return selected, nil
}

func MultiSelect(message string, options []string) ([]string, error) {
	if len(options) == 0 {
		return nil, errors.New("no options available")
	}

	selected := make([]string, 0)
	prompt := &survey.MultiSelect{Message: message, Options: options, PageSize: 20}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return nil, err
	}
	return selected, nil
}
