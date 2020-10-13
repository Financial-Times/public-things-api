# UPP - Public Things API

Provides a publicly available API that gives basic information about things in the graph store. It is written in Go, uses Neo4j as a data store, and is deployed in the UPP Kubernetes stack

## Code

public-things-api

## Primary URL

<https://github.com/Financial-Times/public-things-api>

## Service Tier

Bronze

## Lifecycle Stage

Production

## Delivered By

content

## Supported By

content

## Known About By

- elitsa.pavlova
- kalin.arsov
- ivan.nikolov
- marina.chompalova
- miroslav.gatsanoga

## Host Platform

AWS

## Architecture

Provides a publicly available API that gives basic information about things in the graph store. It is written in Go, uses Neo4j as a data store, and is deployed in the UPP Kubernetes stack

## Contains Personal Data

No

## Contains Sensitive Data

No

## Failover Architecture Type

ActiveActive

## Failover Process Type

FullyAutomated

## Failback Process Type

PartiallyAutomated

## Failover Details

See the [failover guides for more details](https://github.com/Financial-Times/upp-docs/tree/master/failover-guides)

## Data Recovery Process Type

NotApplicable

## Data Recovery Details

The service does not store data, so it does not require any data recovery steps.

## Release Process Type

PartiallyAutomated

## Rollback Process Type

Manual

## Release Details

The release is triggered by making a Github release which is then picked up by a Jenkins multibranch pipeline. The Jenkins pipeline should be manually started in order for it to deploy the helm package to the Kubernetes clusters.

## Key Management Process Type

NotApplicable

## Key Management Details

There is no key rotation procedure for this system.

## Monitoring

Service in UPP K8S delivery clusters:

- Delivery-Prod-EU health: <https://upp-prod-delivery-eu.upp.ft.com/__health/__pods-health?service-name=public-things-api>
- Delivery-Prod-US health: <https://upp-prod-delivery-us.upp.ft.com/__health/__pods-health?service-name=public-things-api>

## First Line Troubleshooting

<https://github.com/Financial-Times/upp-docs/tree/master/guides/ops/first-line-troubleshooting>

## Second Line Troubleshooting

Please refer to the GitHub repository README for troubleshooting information.
