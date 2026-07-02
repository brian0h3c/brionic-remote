========================================================================
  Brionic Remote — portable app folder
========================================================================

This whole folder is the app. Copy it to a USB drive (or anywhere) and
run it on any computer. Nothing gets installed.

HOW TO START
------------
  • macOS    : double-click  "Start-Mac.command"
  • Windows  : double-click  "Start-Windows.bat"
  • Linux    : run           "Start-Linux.sh"

Your web browser opens automatically at http://127.0.0.1:8717.
On first run you create a master password; after that you unlock with
that password (or a registered YubiKey).

WHAT'S IN HERE
--------------
  Start-Mac.command / Start-Windows.bat / Start-Linux.sh
                         launchers for each operating system
  bin/                   the small helper program for each OS/CPU
  brionic-remote.vault   your encrypted data (created on first run)

Everything you save lives ONLY in "brionic-remote.vault", encrypted
with your master password. Keep this folder and that file — that's all
you need to move between computers.

WHY IS THERE A HELPER PROGRAM AND NOT JUST AN HTML FILE?
--------------------------------------------------------
Web browsers are not allowed to open SSH / VNC network connections on
their own (a security sandbox). The tiny helper in "bin/" runs locally,
makes those connections for you, and shows the interface in your
browser. It listens only on your own machine (127.0.0.1) and does not
talk to any outside server.

NOTES
-----
  • macOS/Linux: the launcher makes the helper executable and, on macOS,
    clears the "downloaded from the internet" quarantine flag so it runs
    from a USB drive. If macOS still blocks it, right-click the
    .command file > Open > Open.
  • Windows may warn "unknown publisher" (the helper is unsigned) —
    choose "More info" > "Run anyway".
  • Close the terminal/console window the launcher opens to stop the app.

Open source (MIT): https://github.com/brian0h3c/brionic-remote
