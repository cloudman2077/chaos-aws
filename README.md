# Chaos-AWS

## Abstract
With the popularity of cloud computing, more and more industries choose to manage their own infrastructure and business loads on the public cloud. This puts forward higher requirements for the reliability of cloud providers, and at the same time, new concepts such as infrastructure as code, cloud native, and chaos engineering have begun to take root and land in the industry. This proposal introduces the concept of chaos engineering to the cloud provider to realize fault injection into the cloud provider.

## Background
In 2020, many cloud providers will experience service unavailability and downtime. In this case, many vendors will choose hybrid clouds to reduce their dependence on the cloud. In this context, we hope to simulate the failure of the cloud provider's service and observe the impact of the failure, so as to improve the flexibility and observability of the application.

## Proposal
Provides the fault injection function of the designated service of the cloud service provider, and simulates various situations in which the cloud provider has problems, such as service downtime, insufficient resources, abnormal return, etc., thereby improving the robustness and stability of the system that depends on the cloud provider. Due to the differences between different cloud providers, the goal achieved this time is only to achieve fault injection to AWS, and the most popular services on AWS (s3, ec2, ebs) will be implemented first.

## Goals

- Implement fault injection into AWS designated services
  - Simulate AWS Availability Zone is not available
  - Simulate AWS designated service downtime
  - Insufficient AWS resources, such as inability to create machines, etc.
  - Simulate AWS service returning exception
- No perception of the application, the application itself does not need to make any changes
- Fault recoverable
