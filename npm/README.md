# dhriti (npm)

Install the Dhriti CLI via npm. The postinstall step downloads the platform binary from GitHub Releases.

```bash
npm install -g dhriti
dhriti -v
dhriti login
```

Or without publishing:

```bash
npm install -g git+https://github.com/harshmendhe-arch/dhriti.git#main:npm
```

Requires Node 18+ and network access to GitHub Releases.
