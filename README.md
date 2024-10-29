# Glide Quickstart Project

This is a quickstart project to demonstrate how to use the Glide SDK in a Go application using the Echo web framework.

## Setup Instructions

1. **Install Dependencies:**

    ```bash
    go mod tidy
    ```

2. **Set Environment Variables:**

    ```bash
    export GLIDE_CLIENT_ID=<your-client-id>
    export GLIDE_CLIENT_SECRET=<your-client-secret>
    export GLIDE_REDIRECT_URI=<your-redirect-uri>
    export GLIDE_AUTH_BASE_URL=<auth-base-url>
    export GLIDE_API_BASE_URL=<api-base-url>
    ```

   Replace `<your-client-id>`, `<your-client-secret>`, and other placeholders with your actual credentials.

3. **Run the Application:**

    ```bash
    go run main.go
    ```

4. **Access the Server:**

   Open your browser and navigate to `http://localhost:4567`.

