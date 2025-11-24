# Verification

## Checksums

Verify the checksums for the release:

```sh
KAT_TAG=v0.13.1
curl -LO https://github.com/MacroPower/kat/releases/download/$KAT_TAG/checksums.txt
cosign verify-blob \
  --certificate-identity https://github.com/MacroPower/kat/.github/workflows/release.yaml@refs/tags/$KAT_TAG \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --bundle https://github.com/MacroPower/kat/releases/download/$KAT_TAG/checksums.txt.sigstore.json \
  ./checksums.txt
```

Then, use the checksums to verify any other files from the release:

```sh
sha256sum --ignore-missing -c checksums.txt
```

## Attestations

Verify any artifact with:

```sh
gh attestation verify --owner macropower *.tar.gz
```

## Docker image

```sh
KAT_TAG="v0.13.1"
KAT_IMAGE_TAG="$KAT_TAG-arm64"
cosign verify -o text \
  --certificate-identity https://github.com/MacroPower/kat/.github/workflows/release.yaml@refs/tags/$KAT_TAG \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  ghcr.io/macropower/kat:$KAT_IMAGE_TAG
```

## SBOM

Releases are accompanied by `.sbom.json` files, which can be used with [syft](https://github.com/anchore/syft).

```sh
syft convert *.sbom.json -o syft-table
```

## Notarization

macOS releases are notarized. You can verify the notarization with:

```sh
spctl -a -t install -vv kat
```
