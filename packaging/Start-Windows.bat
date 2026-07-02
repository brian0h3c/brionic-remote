@echo off
rem Brionic Remote - portable launcher for Windows.
rem Double-click this file. It starts the local helper detached (this console
rem closes) and opens your browser. Closing the browser tab shuts it down.
start "" "%~dp0bin\brionic-remote-windows-amd64.exe" --vault "%~dp0brionic-remote.vault" --auto-exit
