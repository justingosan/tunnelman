./AGENTS.md

- ALWAYS USE THE QUICKTODO TASK MANAGEMENT TOOL TO RECORD EVERYTHING YOU DO! If you don't know how to use it, run: `quicktodo context`
- Docs about the tunnel CLI tool is in docs/cloudflare.md
- This repo only contains e2e tests for the tunnelman CLI tool. If you need to run it, read the README_E2E_TESTS.md file for more information.

## Releasing

To create a new release:
1. Commit all changes
2. Create and push a version tag: `git tag -a v1.0.X -m "Release message"`
3. Push with tags: `git push origin master --tags`
4. GitHub Actions will automatically build and release binaries for all platforms

The release workflow creates:
- Linux binaries (amd64/arm64)
- macOS binaries (Intel/Apple Silicon)
- Windows binaries
- Debian packages (.deb)
