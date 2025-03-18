# Vehicle Check Tool

A Go application for checking vehicle information against multiple data sources and publishing results to Google Cloud Pub/Sub.

## Features

- Support for multiple data sources (ACME, Lease Company, Fleet Company, Hire Company)
- Batch processing of VRM checks
- Google Cloud Pub/Sub integration
- Local Pub/Sub emulator support for testing
- Cross-platform support (Windows and Linux)

## Prerequisites

### 1. Go
- **Version**: 1.24.1
- **Installation**:
  - Windows: Download from [Go Downloads](https://golang.org/dl/)
  - Linux: `sudo apt-get install golang-go` (Ubuntu/Debian) or `sudo yum install golang` (RHEL/CentOS)
  - macOS: `brew install go` (using Homebrew)

### 2. Google Cloud SDK
- **Version**: Latest version
- **Installation**:
  - Windows: Download from [Google Cloud SDK Downloads](https://cloud.google.com/sdk/docs/install)
  - Linux: Follow [Linux Installation Guide](https://cloud.google.com/sdk/docs/install-sdk#linux)
  - macOS: `brew install --cask google-cloud-sdk` (using Homebrew)

### 3. Java Runtime Environment (JRE)
- **Version**: 11 or higher
- **Installation**:
  - Windows: Download from [Oracle JDK Downloads](https://www.oracle.com/java/technologies/downloads/) or [OpenJDK](https://adoptium.net/)
  - Linux: `sudo apt-get install default-jre` (Ubuntu/Debian) or `sudo yum install java-11-openjdk` (RHEL/CentOS)
  - macOS: `brew install openjdk@11` (using Homebrew)

### 4. Required Go Packages
After installing Go, run:
```bash
go mod download
```

## Environment Setup

1. Set up Google Cloud credentials:
   ```bash
   gcloud auth application-default login
   ```

2. Set up Pub/Sub emulator:
   ```bash
   gcloud components install pubsub-emulator
   gcloud beta emulators pubsub env-init
   ```

## Running the Application

### Using Command Line Arguments

1. Check a single vehicle:
   ```bash
   go run . -project=test-project -vrm=ABC123 -company=CompanyName
   ```

2. Process a batch file:
   ```bash
   go run . -project=test-project -batch="./batch.json"
   ```

3. Use the Pub/Sub emulator:
   ```bash
   go run . -project=test-project -emulator -vrm=ABC123 -company=CompanyName
   ```

4. Use the Pub/Sub emulator and batch file:
   ```bash
   go run . -project=test-project -emulator -batch="./batch.json"

### Batch File Format
The batch file should be a JSON array of objects with the following structure:
```json
[
  {
    "vrm": "ABC123",
    "company": "CompanyName"
  }
]
```

## Development

### Project Structure
- `main.go`: Main application entry point and flag handling
- `vehicle_check.go`: Core vehicle checking logic
- `data.go`: Data source interface and implementations
- `emulator.go`: Pub/Sub emulator implementation

### Adding New Data Sources
1. Create a new struct implementing the `VehicleDataSource` interface
2. Add the new data source to the `dataSources` slice in `dataa.go`

## Troubleshooting

### Common Issues

1. **Pub/Sub Emulator Not Starting**
   - Ensure Java is installed and in PATH
   - Check if port 8085 is available
   - Verify Google Cloud SDK installation

2. **Authentication Errors**
   - Run `gcloud auth application-default login`
   - Check if credentials are properly set
   - Set GOOGLE_APPLICATION_CREDENTIALS environment variable to the path of your credentials file:
     ```bash
     # Windows
     set GOOGLE_APPLICATION_CREDENTIALS=C:\path\to\credentials.json
     
     # Linux/macOS
     export GOOGLE_APPLICATION_CREDENTIALS=/path/to/credentials.json
     ```

3. **Port Already in Use**
   - The emulator uses port 8085 by default
   - Use `netstat -ano | findstr :8085` (Windows) or `lsof -i :8085` (Linux/macOS) to find processes using the port
   - Kill the process or use a different port
   - To kill the process using port 8085:
     ```bash
     # Windows
     for /f "tokens=5" %a in ('netstat -aon ^| findstr :8085') do taskkill /F /PID %a
     
     # Linux/macOS
     kill -9 $(lsof -ti :8085)
     ```

