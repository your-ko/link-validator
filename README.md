[![Main](https://github.com/your-ko/link-validator/actions/workflows/main.yaml/badge.svg)](https://github.com/your-ko/link-validator/actions/workflows/main.yaml)
[![golangci-lint](https://github.com/your-ko/link-validator/actions/workflows/golangci-lint.yaml/badge.svg)](https://github.com/your-ko/link-validator/actions/workflows/golangci-lint.yaml)
[![Link validation](https://github.com/your-ko/link-validator/actions/workflows/workflow-link-validator.yaml/badge.svg)](https://github.com/your-ko/link-validator/actions/workflows/workflow-link-validator.yaml)

# About
Your project grows and it is time to write some documentation for it?

You already have written some documentation, added links to some modules declared in another repositories or links to 3rd party tools.
Then everything evolves, some repositories decommissioned, some 3rd party tools change their documentation.
And then in the middle of an accident you find yourself in a situation when your documentation is incomplete and some important links leads to nowhere?
Then this is the tool for you! It helps to early detect all broken links in your repository.

## WHat can it do
Link validator can check:
* all GitHub urls (to another repositories)
* links to another websites
* Markdown links to files inside the repository

## Usage
You add this GitHub workflow into your repository:
```yaml
name: Link validator

on:
  workflow_dispatch:
  push:
    branches:
      - master
      - main

env:
  DOCKER_VALIDATOR: ghcr.io/your-ko/link-validator:0.17.0
jobs:
  link-validator:
    name: link-validator
    runs-on: ubuntu-22.04
    steps:
      - name: Checkout
        uses: actions/checkout@v5.0.0

      - name: Run Link validation
        id: lnk
        env:
          LOG_LEVEL: "info"
          FILE_MASKS: "*.md"
          PAT: ${{ secrets.GITHUB_TOKEN }}
        shell: bash
        run: |
          docker run --rm \
            -e LOG_LEVEL \
            -e FILE_MASKS \
            -e LOOKUP_PATH \
            -e EXCLUDE_PATH \
            -e FILE_LIST \
            -e PAT \
            -e CORP_PAT \
            -e CORP_URL \
            -v "${{ github.workspace }}:/work" \
            -w /work \
            ${DOCKER_VALIDATOR}:${DOCKER_VALIDATOR_VERSION}
```


Docker image size is 10Mb
