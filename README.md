# Bash-Runner

A macOS app (written in Go with Wails, HTML/CSS interface) to run `.sh` files with a button click, instead of opening Terminal and typing `bash`.

## Features

- Click **Add file** to pick `.sh` files from any folder on your Mac.
- Each file appears in the list under its display name. The full path is only shown when the file no longer exists.
- Click the **▶** button (Run icon) to run the file with `/bin/bash`. The working directory is the folder that contains the file.
- Click the **⋮** button next to a file to open its menu:
  - **Run with parameters…**: enter parameters (for example `--env staging "new folder"`) to run once, or click **Save** and give it a name to keep that run.
  - **Rename display name**: give the file a different name inside the app. The file name on disk does not change.
  - **Remove from list**: remove the file from the app. The original file is not deleted.
- Saved runs with parameters appear indented right below their file, with their own **▶** button and **⋮** menu (edit name/parameters or delete).
- Output (stdout and stderr) appears in the panel below, along with the exit code when the run finishes.
- The list and display names are saved between app launches.

## Requirements

- macOS
- Go 1.26.5 or later (as declared in `go.mod`)
- Xcode Command Line Tools (Wails needs cgo): `xcode-select --install`

## Build the macOS app

```bash
./build.sh
```

The [build.sh](build.sh) script installs the `wails` tool if it is missing, then packages the app as `Bash Runner.app` in the project folder. Drag this file into the Applications folder if you want to keep it.

To run in development mode (reloads automatically when `frontend/` changes):

```bash
wails dev
```

The app icon is [icon.png](icon.png).
