package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"time"

	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/tools"
)

func registerTools(r *tools.Registry) {
	r.Register(weatherTool())
	r.Register(timeTool())
	r.Register(httpGetTool())
	r.Register(jokeAPITool())
}

// weatherTool calls Open-Meteo API (free, no key required)
func weatherTool() *tools.Tool {
	return tools.NewTool("get_weather").
		Description("Get current weather for a location. Returns temperature, conditions, and wind.").
		StringParam("city", "City name (e.g., Paris, London, Tokyo)", true).
		Handler(func(ctx context.Context, input map[string]any) (any, error) {
			city := input["city"].(string)

			// First, geocode the city using Open-Meteo's geocoding API
			geoURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1", url.QueryEscape(city))

			geoResp, err := httpGet(ctx, geoURL)
			if err != nil {
				return nil, fmt.Errorf("geocoding failed: %w", err)
			}

			results, ok := geoResp["results"].([]any)
			if !ok || len(results) == 0 {
				return map[string]any{"error": "city not found", "city": city}, nil
			}

			loc := results[0].(map[string]any)
			lat := loc["latitude"].(float64)
			lon := loc["longitude"].(float64)
			name := loc["name"].(string)
			country := loc["country"].(string)

			// Now get weather
			weatherURL := fmt.Sprintf(
				"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&current=temperature_2m,relative_humidity_2m,weather_code,wind_speed_10m",
				lat, lon,
			)

			weatherResp, err := httpGet(ctx, weatherURL)
			if err != nil {
				return nil, fmt.Errorf("weather API failed: %w", err)
			}

			current := weatherResp["current"].(map[string]any)

			return map[string]any{
				"location":    fmt.Sprintf("%s, %s", name, country),
				"temperature": current["temperature_2m"],
				"unit":        "celsius",
				"humidity":    current["relative_humidity_2m"],
				"wind_speed":  current["wind_speed_10m"],
				"wind_unit":   "km/h",
				"condition":   weatherCodeToString(int(current["weather_code"].(float64))),
			}, nil
		}).
		Build()
}

// timeTool returns current time (no external API needed)
func timeTool() *tools.Tool {
	return tools.NewTool("get_time").
		Description("Get current time for a timezone").
		StringParam("timezone", "IANA timezone (e.g., Europe/Paris, America/New_York, Asia/Tokyo)", true).
		Handler(func(ctx context.Context, input map[string]any) (any, error) {
			tz := input["timezone"].(string)
			loc, err := time.LoadLocation(tz)
			if err != nil {
				return nil, fmt.Errorf("invalid timezone: %s", tz)
			}
			now := time.Now().In(loc)
			return map[string]any{
				"timezone": tz,
				"time":     now.Format("15:04:05"),
				"date":     now.Format("Monday, January 2, 2006"),
				"unix":     now.Unix(),
			}, nil
		}).
		Build()
}

// httpGetTool is a generic HTTP GET tool (be careful with this in production!)
func httpGetTool() *tools.Tool {
	return tools.NewTool("http_get").
		Description("Fetch data from a URL. Use for public APIs only.").
		StringParam("url", "The URL to fetch", true).
		Handler(func(ctx context.Context, input map[string]any) (any, error) {
			targetURL := input["url"].(string)

			// Basic URL validation / allowlist (expand as needed)
			parsed, err := url.Parse(targetURL)
			if err != nil {
				return nil, fmt.Errorf("invalid URL: %w", err)
			}

			// Simple allowlist - expand for your use case
			allowed := map[string]bool{
				"api.open-meteo.com":            true,
				"geocoding-api.open-meteo.com":  true,
				"official-joke-api.appspot.com": true,
				"api.github.com":                true,
				"httpbin.org":                   true,
			}

			if !allowed[parsed.Host] {
				return nil, fmt.Errorf("domain not allowed: %s", parsed.Host)
			}

			return httpGet(ctx, targetURL)
		}).
		Build()
}

// jokeAPITool calls a public joke API
func jokeAPITool() *tools.Tool {
	return tools.NewTool("get_joke").
		Description("Get a random joke to lighten the mood").
		Handler(func(ctx context.Context, input map[string]any) (any, error) {
			resp, err := httpGet(ctx, "https://official-joke-api.appspot.com/random_joke")
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"setup":     resp["setup"],
				"punchline": resp["punchline"],
			}, nil
		}).
		Build()
}

// Helper functions

func httpGet(ctx context.Context, url string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "agent-example/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func weatherCodeToString(code int) string {
	// WMO Weather interpretation codes
	// https://open-meteo.com/en/docs
	switch {
	case code == 0:
		return "clear sky"
	case code <= 3:
		return "partly cloudy"
	case code <= 49:
		return "foggy"
	case code <= 59:
		return "drizzle"
	case code <= 69:
		return "rain"
	case code <= 79:
		return "snow"
	case code <= 84:
		return "rain showers"
	case code <= 94:
		return "snow showers"
	case code <= 99:
		return "thunderstorm"
	default:
		return "unknown"
	}
}
