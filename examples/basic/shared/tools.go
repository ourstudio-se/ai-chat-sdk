package shared

import (
	"context"
	"strings"

	"github.com/ourstudio-se/ai-chat-sdk/tools"
)

// Product represents a product in our database.
type Product struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       float64  `json:"price"`
	Features    []string `json:"features"`
}

// Inventory represents stock information.
type Inventory struct {
	ProductCode string `json:"productCode"`
	InStock     bool   `json:"inStock"`
	Quantity    int    `json:"quantity"`
}

// Simulated database
var productDB = map[string]Product{
	"WIDGET-PRO": {
		Code:        "WIDGET-PRO",
		Name:        "Widget Pro X1000",
		Description: "Our flagship widget with advanced features",
		Price:       299.99,
		Features:    []string{"Wireless", "Waterproof", "Long battery"},
	},
	"GADGET-MINI": {
		Code:        "GADGET-MINI",
		Name:        "Gadget Mini",
		Description: "Compact and portable gadget",
		Price:       49.99,
		Features:    []string{"Compact", "USB-C", "Lightweight"},
	},
}

var inventoryDB = map[string]Inventory{
	"WIDGET-PRO":  {ProductCode: "WIDGET-PRO", InStock: true, Quantity: 50},
	"GADGET-MINI": {ProductCode: "GADGET-MINI", InStock: false, Quantity: 0},
}

// RegisterTools registers all tools with the registry.
func RegisterTools(registry *tools.Registry) {
	// Source: Get product details (read-only)
	registry.RegisterSource("get_product", tools.Source{
		Description: "Get product details by code or search term",
		Params: tools.Params{
			"code":   tools.String("Product code", false),
			"search": tools.String("Search term", false),
		},
		Fetch: func(ctx context.Context, p tools.Input) (any, error) {
			if code := p.String("code"); code != "" {
				if product, ok := productDB[code]; ok {
					return product, nil
				}
				return nil, nil
			}
			// Simple search - return all matching products
			var results []Product
			search := p.String("search")
			for _, product := range productDB {
				if containsIgnoreCase(product.Name, search) ||
					containsIgnoreCase(product.Description, search) {
					results = append(results, product)
				}
			}
			return results, nil
		},
	})

	// Source: Get inventory status (read-only)
	registry.RegisterSource("get_inventory", tools.Source{
		Description: "Check inventory/stock status for a product",
		Params: tools.Params{
			"product_code": tools.String("Product code to check", true),
		},
		Fetch: func(ctx context.Context, p tools.Input) (any, error) {
			code := p.String("product_code")
			if inv, ok := inventoryDB[code]; ok {
				return inv, nil
			}
			return Inventory{ProductCode: code, InStock: false, Quantity: 0}, nil
		},
	})

	// Action: Create support ticket (has side effects)
	registry.RegisterAction("create_ticket", tools.Action{
		Description: "Create a support ticket for the customer",
		Params: tools.Params{
			"subject":      tools.String("Ticket subject", true),
			"description":  tools.String("Issue description", true),
			"product_code": tools.String("Related product code", false),
			"priority":     tools.Enum("Priority level", []string{"low", "medium", "high"}, false),
		},
		Execute: func(ctx context.Context, p tools.Input) (any, error) {
			// In real app: create ticket in database/external system
			return map[string]any{
				"ticketId": "TKT-12345",
				"status":   "created",
				"subject":  p.String("subject"),
			}, nil
		},
		RequiresConfirmation: true,
	})
}

func containsIgnoreCase(s, substr string) bool {
	if substr == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
