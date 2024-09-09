# Yaku Helm Chart

This repository contains the Helm chart for Yaku. It consists of the subcharts for the following components:
- Argo Workflows
- Minio
- Core-API
- Yaku-UI

## Release Process

### Purpose of the Release

Before releasing a new version, the purpose of the release should be considered. It is one of the following:

1. Chart related changes

   For this, we create a new patch increment version of the chart.

   This release is done after chart changes that are required for our deployments or chart changes in general that the customer will use in the next customer release.

   We keep track of these chart related changes in [HELM_CHANGELOG.md file](./HELM_CHANGELOG.md). So when creating a new version, update that file with your changes.


2. Official release version for customers

   For this, we create a new minor (or can be major) increment version of the chart and generate the release note of Yaku components (api, ui, onyx and apps).


### Release a New Chart Version

Based on the purpose of the new version, follow the steps below to release a new version of the chart:


1. The *Create release PR* workflow should be manually triggered with the following inputs:
   - `version_inc`:

      The version increment type, which can be one of the following: `major`, `minor`, `patch`.
      Default is `patch`. This means you don't need to enter components versions and no release notes will be created.
      If you are releasing a customer version, choose `minor` or `major`based on corresponding Yaku components changes.
   - `core-api-version`: 

      The version of yaku-core-api to be included in the new release.
      Mandatory for customer releases.
   - `core-version`: 
   
      The version of yaku-core image to be included in the new release.
      Mandatory for customer releases.

   - `ui-version`: 
   
      The version of the yaku-ui to be included in the new release.
      Mandatory for customer releases.

   If it is a customer release, a changelog will be created with all the changes from all included components. A PR will be created with the changelog and the updated versions.

   **IMPORTANT:** Before merging the PR, ensure it is polished and presentable.

   If it is an internal release, a PR will be created with the new version.

   

2. Once the created PR is merged, the *Release chart* workflow should be manually triggered with the following inputs:
   - `version`: The version of the Helm chart to be released, as found in the `Chart.yaml` file.
   - `overwrite`: If set to `true`, the workflow will overwrite the existing Helm chart with the same version in ACR.

3. Once the workflow is completed and the GitHub release is created, the *Publish chart* workflow will be triggered automatically. This workflow will publish the Helm chart to ACR.

A diagram of the release process can be found [in the wiki](https://inside-docupedia.bosch.com/confluence/display/GROWPAT/Release+of+on-prem+deployment).

### Pull Helm Chart from ACR

1. Get access to ACR and login
2. Pull the helm chart, e.g. with:
   ```sh
   helm pull oci://growpatcr.azurecr.io/helm/yaku --version <your-version>
   ```

To continue, you can find some more documentation [here](./documentation).

