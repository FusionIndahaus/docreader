# Start Docreader Microservice
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Starting Docreader Microservice" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Check if .env file exists
if (-not (Test-Path ".env")) {
    Write-Host "❌ Error: .env file not found!" -ForegroundColor Red
    Write-Host "Please copy .env.example to .env and configure it." -ForegroundColor Yellow
    exit 1
}

Write-Host "✅ Found .env file" -ForegroundColor Green

# Check Go installation
try {
    $goVersion = go version
    Write-Host "✅ Go is installed: $goVersion" -ForegroundColor Green
} catch {
    Write-Host "❌ Error: Go is not installed!" -ForegroundColor Red
    Write-Host "Please install Go from https://go.dev/dl/" -ForegroundColor Yellow
    exit 1
}

Write-Host ""
Write-Host "Checking if port 8081 is already in use..." -ForegroundColor Yellow

# Check if port 8081 is already in use
$existingProcess = Get-NetTCPConnection -LocalPort 8081 -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess -Unique

if ($existingProcess) {
    Write-Host "⚠️  Port 8081 is already in use by process $existingProcess" -ForegroundColor Yellow
    Write-Host "Stopping existing process..." -ForegroundColor Yellow
    try {
        Stop-Process -Id $existingProcess -Force
        Start-Sleep -Seconds 1
        Write-Host "✅ Existing process stopped" -ForegroundColor Green
    } catch {
        Write-Host "❌ Error: Failed to stop existing process" -ForegroundColor Red
        Write-Host "Please manually stop the process with PID $existingProcess" -ForegroundColor Yellow
        exit 1
    }
}

Write-Host ""
Write-Host "Starting service..." -ForegroundColor Yellow
Write-Host ""

# Run the service
go run .
