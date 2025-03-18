# Find the process ID (PID) associated with port 8085
$process = Get-NetTCPConnection -LocalPort 8085 -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess

# Check if a process was found
if ($process) {
    try {
        # Stop the process using the found PID
        Stop-Process -Id $process -Force -ErrorAction Stop
        Write-Host "Process using port 8085 (PID: $process) stopped successfully."
    }
    catch {
        Write-Error "Failed to stop process using port 8085. Error: $($_.Exception.Message)"
    }
} else {
    Write-Host "No process found using port 8085."
}