# winget manifests for Dhriti
#
# After a GitHub Release exists, submit these to microsoft/winget-pkgs:
#   https://github.com/microsoft/winget-pkgs
#
# Replace VERSION and the SHA256 values from release checksums.txt.
# Folder layout follows winget-pkgs conventions:
#   manifests/h/harshmendhe-arch/dhriti/<VERSION>/

PackageIdentifier: harshmendhe-arch.dhriti
PackageVersion: VERSION
DefaultLocale: en-US
PackageName: Dhriti
PackageUrl: https://github.com/harshmendhe-arch/dhriti
License: MIT
ShortDescription: Terminal-based AI assistant with WebSocket gateway support
Installers:
  - Architecture: x64
    InstallerType: zip
    InstallerUrl: https://github.com/harshmendhe-arch/dhriti/releases/download/vVERSION/dhriti-windows-x86_64.zip
    InstallerSha256: REPLACE_SHA256
    NestedBinaries:
      - RelativeFilePath: dhriti.exe
        PortableCommandAlias: dhriti
ManifestType: singleton
ManifestVersion: 1.6.0
