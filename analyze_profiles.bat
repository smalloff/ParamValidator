@echo off
setlocal enabledelayedexpansion
chcp 65001 >nul

:menu
cls
echo === Profile Analysis Menu ===
echo.
echo 1 - Analyze CPU profile
echo 2 - Analyze Memory profile
echo 3 - Launch web interface
echo 4 - Create test profiles
echo 5 - Exit
echo.

set /p choice="Select option: "

if "!choice!"=="1" goto analyze_cpu
if "!choice!"=="2" goto analyze_mem
if "!choice!"=="3" goto web_interface
if "!choice!"=="4" goto create_test
if "!choice!"=="5" goto exit

goto menu

:analyze_cpu
if not exist "cpu.prof" (
    echo File cpu.prof not found!
    pause
    goto menu
)

echo Analyzing CPU profile...
go tool pprof -top cpu.prof > cpu_results.txt
type cpu_results.txt

echo.
echo Results saved to cpu_results.txt
pause
goto menu

:analyze_mem
if not exist "heap.prof" (
    echo File heap.prof not found!
    pause
    goto menu
)

echo Analyzing Memory profile...
go tool pprof -top -alloc_objects heap.prof > mem_results.txt
type mem_results.txt

echo.
echo Results saved to mem_results.txt
pause
goto menu

:web_interface
if not exist "cpu.prof" (
    echo File cpu.prof not found!
    pause
    goto menu
)

echo Launching web interface on http://localhost:8080
start go tool pprof -http=:8080 cpu.prof
goto menu

:create_test
echo Creating test application...
(
echo package main
echo.
echo import (
echo     "os"
echo     "runtime/pprof"
echo     "time"
echo )
echo.
echo func main() {
echo     // CPU profiling
echo     cpuFile, _ := os.Create("cpu.prof")
echo     pprof.StartCPUProfile(cpuFile)
echo     defer pprof.StopCPUProfile()
echo     defer cpuFile.Close()
echo.
echo     // Test workload
echo     for i := 0; i < 1000; i++ {
echo         fastOperation()
echo         if i%%100 == 0 {
echo             slowOperation()
echo         }
echo     }
echo.
echo     // Memory profiling
echo     memFile, _ := os.Create("heap.prof")
echo     defer memFile.Close()
echo     pprof.WriteHeapProfile(memFile)
echo.
echo     println("Profiling completed!")
echo }
echo.
echo func slowOperation() {
echo     time.Sleep(50 * time.Millisecond)
echo     // Create some memory allocations
echo     _ = make([]byte, 1024*1024) // 1MB
echo }
echo.
echo func fastOperation() {
echo     time.Sleep(5 * time.Millisecond)
echo }
) > test_app.go

echo Compiling...
go build -o test_app.exe test_app.go

echo Running with profiling...
test_app.exe
echo Test profiles created!

del test_app.go test_app.exe 2>nul
pause
goto menu

:exit
echo Exiting...
exit