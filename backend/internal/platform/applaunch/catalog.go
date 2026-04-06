package applaunch

import (
	"slices"
	"strings"
)

const (
	BudgetAppID = "budget"
)

var budgetAccessRoles = []string{"app_budget_access"}

type Definition struct {
	ID            string
	Name          string
	Description   string
	Icon          string
	Href          string
	Status        string
	CategoryID    string
	CategoryTitle string
	AccessRoles   []string
}

type App struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon"`
	Href        string `json:"href"`
	Status      string `json:"status,omitempty"`
}

type Category struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Apps  []App  `json:"apps"`
}

func Catalog(budgetAppURL string) []Definition {
	href := strings.TrimSpace(budgetAppURL)
	if href == "" {
		href = "/budget"
	}

	return []Definition{
		{
			ID:            BudgetAppID,
			Name:          "Budget Management",
			Icon:          "coins",
			Href:          href,
			CategoryID:    "acquisti",
			CategoryTitle: "Acquisti",
			AccessRoles:   BudgetAccessRoles(),
		},
	}
}

func BudgetAccessRoles() []string {
	return slices.Clone(budgetAccessRoles)
}

func VisibleCategories(definitions []Definition, roles []string) []Category {
	categories := make([]Category, 0)
	categoryIdx := make(map[string]int)

	for _, definition := range definitions {
		if !hasAnyRole(roles, definition.AccessRoles) {
			continue
		}

		idx, ok := categoryIdx[definition.CategoryID]
		if !ok {
			categories = append(categories, Category{
				ID:    definition.CategoryID,
				Title: definition.CategoryTitle,
				Apps:  []App{},
			})
			idx = len(categories) - 1
			categoryIdx[definition.CategoryID] = idx
		}

		categories[idx].Apps = append(categories[idx].Apps, App{
			ID:          definition.ID,
			Name:        definition.Name,
			Description: definition.Description,
			Icon:        definition.Icon,
			Href:        definition.Href,
			Status:      definition.Status,
		})
	}

	return categories
}

func hasAnyRole(userRoles []string, requiredRoles []string) bool {
	if len(requiredRoles) == 0 {
		return true
	}

	for _, role := range requiredRoles {
		if slices.Contains(userRoles, role) {
			return true
		}
	}

	return false
}
