# Stop Docreader Microservice
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Stopping Docreader Microservice" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Find process on port 8081
$process = Get-NetTCPConnection -LocalPort 8081 -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess -Unique

if ($process) {
    Write-Host "Found process $process on port 8081" -ForegroundColor Yellow
    Write-Host "Stopping process..." -ForegroundColor Yellow
    
    try {
        Stop-Process -Id $process -Force
        Write-Host "✅ Service stopped successfully" -ForegroundColor Green
    } catch {
        Write-Host "❌ Error: Failed to stop process" -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host "ℹ️  No service running on port 8081" -ForegroundColor Cyan
}

Write-Host ""
