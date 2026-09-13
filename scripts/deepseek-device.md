# DeepSeek device initialization

`deepseek-device.mjs` obtains a device ID through the original SDK on DeepSeek's
sign-in page. It opens an installed Chrome, Chromium, or Edge with a new temporary
profile, waits for the registered `B`-prefixed identifier, then closes that browser
and removes its profile. Registration needs a browser and network access, but no
account password. The helper uses Node.js 22+ built-ins and has no npm dependencies.

Create a private output directory, then run:

```sh
mkdir -p .tmp
node scripts/deepseek-device.mjs --output .tmp/deepseek-device-id
```

The file contains the raw identifier followed by one newline. New files use mode
`0600` on POSIX; on Windows, choose a private directory with suitable Windows
permissions. Existing files and symlinks are rejected. Omitting `--output` writes
the identifier to stdout. Progress and errors always go to stderr, and a successful
run reports only the identifier's length there.

Use the file's value as `DS2API_DEEPSEEK_DEVICE_ID` for local, container, or Vercel
deployments. Treat it as private device metadata and keep it out of commits and
shared logs. Device registration does not authenticate an account or guarantee
that a later login passes DeepSeek's checks; the account still needs valid login
credentials. If the website rejects the device or requires verification, complete
the normal website flow before trying again.

The default registration deadline is 120 seconds. `--timeout 30` changes it to
30 seconds; values from 10 through 300 are accepted. Cleanup may take a few
additional seconds. The script stops on verification pages it can identify and
reports other SDK or network failures when the deadline expires. It uses the
website's SDK unchanged, with a visible browser and its actual environment.

Set `DS2API_BROWSER_PATH` to an installed browser executable if automatic detection
does not find one. For example, on macOS:

```sh
DS2API_BROWSER_PATH='/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge' \
  node scripts/deepseek-device.mjs --output .tmp/deepseek-device-id
```

On Windows, set the same environment variable to the full `chrome.exe` or
`msedge.exe` path. The script searches common installation directories on macOS
and Windows, and executable names on `PATH` on all platforms. A Linux desktop
session is required to display the normal browser.

Independent regression checks:

```sh
node --test scripts/deepseek-device.test.mjs
```
