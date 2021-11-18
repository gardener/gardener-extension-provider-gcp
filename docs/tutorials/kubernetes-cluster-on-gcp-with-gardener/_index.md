---
title: Create a Кubernetes cluster on GCP with Gardener
level: beginner
category: Getting Started
scope: app-developer
---

### Overview

Gardener allows you to create a Kubernetes cluster on different infrastructure providers. This tutorial will guide you through the process of creating a cluster on GCP.

### Prerequisites

- You have created a [GCP account](https://console.cloud.google.com/).
- You have access to the Gardener dashboard and have permissions to create projects.

### Steps

1. Go to the Gardener dashboard and create a *Project*.

    <img src="images/new-gardener-project.png">

1. Check which roles are required by Gardener.

    1. Choose *Secrets*, then the plus icon <img src="images/plus-icon.png"> and select *GCP*.

        <img src="images/create-secret-gcp.png">

    1. Click on the help button <img src="images/help-icon.png">.

        <img src="images/gardener-gcp-secret-1.png">

        <img src="images/gardener-gcp-secret-2.png">

1. Create a service account with the correct roles in GCP:
    1. [Create a new service account in GCP](https://console.cloud.google.com/iam-admin/serviceaccounts).

        <img src="images/gcp-create-service-account-0.png">

    1. Enter the name and description of your service account.

    1. Assign the roles required by Gardener.
    1. Choose *Done*.

        <img src="images/gcp-create-service-account-1.png">

4. Create a key for your service:

    1. Locate your service account, then choose *Actions* and *Manage keys*.

        <img src="images/gcp-create-key-0.png">

    1. Choose *Add Key*, then *Create new key*.

        <img src="images/gcp-create-key-1.png">

    1. Save the private key of the service account in JSON format.

        <img src="images/gcp-create-key-2.png">
        <img src="images/gcp-create-key-3.png">

        > Note: Save the key of the user, it’s used later to create secrets for Gardener.

1. Enable the [Google Compute API](https://console.developers.google.com/apis/library/compute.googleapis.com) by following [these steps](https://cloud.google.com/endpoints/docs/openapi/enable-api).
    > When you are finished, you should see the following page:
    
    <img src="images/gcp-compute-engine-api.png">


1. Enable the [Google IAM API](https://console.developers.google.com/apis/library/iam.googleapis.com) by following [these steps](https://cloud.google.com/endpoints/docs/openapi/enable-api).
    > When you are finished, you should see the following page:

    <img src="images/gcp-iam-api.png">

1. On the Gardener dashboard, choose *Secrets* and then the plus sign <img src="images/plus-icon.png">. Select *GCP* from the drop down menu to add a new GCP secret.

1. Create your secret.

    1. Type the name of your secret.
    1. Select your *Cloud Profile*.
    1. Copy and paste the contents of the *.JSON* file you saved when you created the secret key on GCP.
    3. Choose *Add secret*.
    <img src="images/add-gcp-secret.png">

    > After completing these steps, you should see your newly created secret in the *Infrastructure Secrets* section.

    <img src="images/secret-stored.png">


1. To create a new cluster, choose *Clusters* and then the plus sign in the upper right corner.

    <img src="images/new-cluster.png">

1. In the *Create Cluster* section:
    1. Select *GCP* in the *Infrastructure* tab.
    1. Type the name of your cluster in the *Cluster Details* tab.
    1. Choose the secret you created before in the *Infrastructure Details* tab.
    1. Choose *Create*.

    <img src="images/create-cluster.png">

1. Wait for your cluster to get created.

    <img src="images/processing-cluster.png">

### Result

After completing the steps in this tutorial, you will be able to see and download the kubeconfig of your cluster.

  <img src="images/copy-kubeconfig.png">
