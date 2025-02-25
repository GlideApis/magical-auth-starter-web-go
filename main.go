package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/GlideApis/sdk-go/pkg/glide"
	"github.com/GlideApis/sdk-go/pkg/types"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// SessionData stores information about an authentication session
type SessionData struct {
	PhoneNumber     string `json:"phoneNumber"`
	Status          string `json:"status"`
	DeviceIPAddress string `json:"deviceIpAddress"`
	Error           string `json:"error,omitempty"`
}

// getClientIP extracts the client IP address from the request headers
func getClientIP(c echo.Context) string {
	// Check X-Forwarded-For header first
	if xForwardedFor := c.Request().Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		// If multiple IPs in X-Forwarded-For, take the first one (original client)
		if i := strings.Index(xForwardedFor, ","); i > 0 {
			return strings.TrimSpace(xForwardedFor[:i])
		}
		return strings.TrimSpace(xForwardedFor)
	}
	// Fall back to RemoteAddr if X-Forwarded-For is not present
	return c.Request().RemoteAddr
}

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

	// State cache to store session data
	stateCache := make(map[string]*SessionData)
	var stateCacheMutex sync.Mutex // To handle concurrent access

	// Create Echo instance
	e := echo.New()

	// 	e.Use(middleware.Logger())
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
		deviceIPAddress := getClientIP(c)
		log.Printf("Start Auth for %s from IP %s", phoneNumber, deviceIPAddress)

		// Generate a new session ID
		sessionID := uuid.New().String()

		// Store session data in state cache
		stateCacheMutex.Lock()
		stateCache[sessionID] = &SessionData{
			PhoneNumber:     phoneNumber,
			Status:          "pending",
			DeviceIPAddress: deviceIPAddress,
		}
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
			DeviceIPAddress: deviceIPAddress,
		}, types.ApiConfig{
			SessionIdentifier: sessionID,
		})
		if err != nil {
			log.Printf("Error starting auth: %v", err)
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Error starting auth"})
		}

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
		deviceIPAddress := getClientIP(c)
		log.Printf("Check Auth for %s from IP %s", phoneNumber, deviceIPAddress)

		// Call glideClient.MagicAuth.VerifyAuth
		verifyRes, err := glideClient.MagicAuth.VerifyAuth(types.MagicAuthVerifyProps{
			PhoneNumber: phoneNumber,
			Token:       token,
			DeviceIPAddress: deviceIPAddress,
		}, types.ApiConfig{
			SessionIdentifier: sessionID,
		})
		if err != nil {
			log.Printf("Error verifying token: %v", err)
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Error verifying token"})
		}

		// Update session status
		stateCacheMutex.Lock()
		if session, exists := stateCache[sessionID]; exists {
			if verifyRes.Verified {
				session.Status = "verified"
			} else {
				session.Status = "failed"
			}
		}
		stateCacheMutex.Unlock()

		return c.JSON(http.StatusOK, verifyRes)
	})

	e.POST("/api/get-session", func(c echo.Context) error {
		var body struct {
			State string `json:"state"`
		}
		if err := c.Bind(&body); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		}

		state := body.State
		log.Println("Get Session")

		// Retrieve session data from state cache
		stateCacheMutex.Lock()
		sessionData, ok := stateCache[state]
		stateCacheMutex.Unlock()

		if !ok {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Session not found"})
		}
		return c.JSON(http.StatusOK, sessionData)
	})

	// Add callback endpoint
	e.GET("/callback", func(c echo.Context) error {
		state := c.QueryParam("state")
		errorParam := c.QueryParam("error")

		stateCacheMutex.Lock()
		session, exists := stateCache[state]
		if !exists {
			stateCacheMutex.Unlock()
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid state parameter"})
		}

		if errorParam != "" {
			session.Status = "error"
			session.Error = errorParam
		} else {
			session.Status = "callback_received"
		}
		stateCacheMutex.Unlock()

		return c.File("static/index.html")
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

	return types.GlideSdkSettings{
		ClientID:     os.Getenv("GLIDE_CLIENT_ID"),
		ClientSecret: os.Getenv("GLIDE_CLIENT_SECRET"),
		RedirectURI:  os.Getenv("GLIDE_REDIRECT_URI"),
		Internal: types.InternalSettings{
			AuthBaseURL: os.Getenv("GLIDE_AUTH_BASE_URL"),
			APIBaseURL:  os.Getenv("GLIDE_API_BASE_URL"),
			// optional add here the log level (LogLevel: types.DEBUG)
		},
	}
}
