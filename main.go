package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/GlideApis/sdk-go/pkg/glide"
	"github.com/GlideApis/sdk-go/pkg/types"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// Load environment variables from .env file if it exists
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found or error reading .env file")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "4567"
	}

	// Initialize the GlideClient
	settings := SetupGlideSettings()
	glideClient, err := glide.NewGlideClient(settings)
	if err != nil {
		log.Fatalf("Failed to create GlideClient: %v", err)
	}

	// State cache to store session IDs and phone numbers
	stateCache := make(map[string]string)
	var stateCacheMutex sync.Mutex // To handle concurrent access

	// Create Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit("2M"))

	// Serve static files
	e.Static("/", "static")

	// Routes
	e.GET("/", func(c echo.Context) error {
		return c.File("static/index.html")
	})

	e.POST("/api/start-verification", func(c echo.Context) error {
		// Parse request body
		var body struct {
			PhoneNumber string `json:"phoneNumber"`
		}
		if err := c.Bind(&body); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		}

		phoneNumber := body.PhoneNumber

		// Generate a new session ID
		sessionID := uuid.New().String()

		// Store phone number in state cache
		stateCacheMutex.Lock()
		stateCache[sessionID] = phoneNumber
		stateCacheMutex.Unlock()

		// Call glideClient.MagicAuth.StartAuth
		redirectURL := os.Getenv("MAGIC_REDIRECT_URI")
		if redirectURL == "" {
			redirectURL = fmt.Sprintf("http://localhost:%s/", port)
		}

		authRes, err := glideClient.MagicAuth.StartAuth(types.MagicAuthStartProps{
			PhoneNumber: phoneNumber,
			State:       sessionID,
			RedirectURL: redirectURL,
		}, types.ApiConfig{
			SessionIdentifier: sessionID,
		})
		if err != nil {
			log.Println("Error starting auth:", err)
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Error starting auth"})
		}
		log.Println("authRes", authRes)

		// Return authRes and sessionID to the client
		return c.JSON(http.StatusOK, authRes)
	})

	e.POST("/api/check-verification", func(c echo.Context) error {
		// Parse request body
		var body struct {
			PhoneNumber string `json:"phoneNumber"`
			Token       string `json:"token"`
			SessionID   string `json:"sessionId"`
		}
		if err := c.Bind(&body); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		}

		phoneNumber := body.PhoneNumber
		token := body.Token
		sessionID := body.SessionID

		// Call glideClient.MagicAuth.VerifyAuth
		verifyRes, err := glideClient.MagicAuth.VerifyAuth(types.MagicAuthVerifyProps{
			PhoneNumber: phoneNumber,
			Token:       token,
		}, types.ApiConfig{
			SessionIdentifier: sessionID,
		})
		if err != nil {
			log.Println("Error verifying token:", err)
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Error verifying token"})
		}

		// 		// Report metric
		// 		reportMetric(types.Metric{
		// 			SessionID:  sessionID,
		// 			Timestamp:  time.Now(),
		// 			MetricName: "third party success",
		// 			API:        "magic-auth",
		// 			ClientID:   "AGGX1YZ8524ZZDIKMOEQZ99",
		// 		})

		return c.JSON(http.StatusOK, verifyRes)
	})

	e.POST("/api/get-session", func(c echo.Context) error {
		// Parse request body
		var body struct {
			State string `json:"state"`
		}
		if err := c.Bind(&body); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		}

		state := body.State

		// Retrieve phone number from state cache
		stateCacheMutex.Lock()
		phoneNumber, ok := stateCache[state]
		stateCacheMutex.Unlock()

		if !ok {
			log.Println("State not found")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Error getting session"})
		}

		return c.JSON(http.StatusOK, map[string]string{"phoneNumber": phoneNumber})
	})

	// Start server
	e.Logger.Printf("Server is running on http://localhost:%s", port)
	e.Logger.Fatal(e.Start(":" + port))
}

// SetupGlideSettings initializes the Glide SDK settings
func SetupGlideSettings() types.GlideSdkSettings {
	if os.Getenv("GLIDE_CLIENT_ID") == "" {
		log.Fatal("GLIDE_CLIENT_ID environment variable is not set")
	}
	if os.Getenv("GLIDE_CLIENT_SECRET") == "" {
		log.Fatal("GLIDE_CLIENT_SECRET environment variable is not set")
	}
	// 	if os.Getenv("GLIDE_REDIRECT_URI") == "" {
	// 		log.Info("GLIDE_REDIRECT_URI environment variable is not set")
	// 	}
	// 	if os.Getenv("GLIDE_AUTH_BASE_URL") == "" {
	// 		log.Info("GLIDE_AUTH_BASE_URL environment variable is not set")
	// 	}
	// 	if os.Getenv("GLIDE_API_BASE_URL") == "" {
	// 		log.Info("GLIDE_API_BASE_URL environment variable is not set")
	// 	}
	// 	if os.Getenv("REPORT_METRIC_URL") == "" {
	// 		fmt.Info("REPORT_METRIC_URL environment variable is not set")
	// 	}
	return types.GlideSdkSettings{
		ClientID:     os.Getenv("GLIDE_CLIENT_ID"),
		ClientSecret: os.Getenv("GLIDE_CLIENT_SECRET"),
		RedirectURI:  os.Getenv("GLIDE_REDIRECT_URI"),
		Internal: types.InternalSettings{
			AuthBaseURL: os.Getenv("GLIDE_AUTH_BASE_URL"),
			APIBaseURL:  os.Getenv("GLIDE_API_BASE_URL"),
		},
	}
}
