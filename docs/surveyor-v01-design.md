# **SURVEYOR**

**Continuous Compliance Evidence Platform — v0.1 Design Document**

## **1\. Overview**

Surveyor is an open-source compliance evidence platform built on the premise that genuine compliance requires verifiable proof of system state — not generated documents, filled-in templates, or rubber-stamped reports. Evidence collected by Surveyor is cryptographically signed, tamper-resistant, and continuously sampled, providing an auditable chain of custody that can be independently verified without trusting Surveyor's own infrastructure.

The platform is designed for infrastructure and security engineers operating in regulated environments — the kinds of teams that need to demonstrate continuous compliance to SOC 2, FedRAMP, ISO 27001, and similar frameworks without the overhead of a full-time compliance consulting engagement.

### **1.1 Core Thesis**

Compliance tooling has long confused documentation for evidence. A policy document asserts that a control exists. A signed, timestamped snapshot of actual system state proves it. Surveyor collects the latter.

The trust model is trustless by design: the verification chain — artifact hash, cryptographic signature, transparency log inclusion proof — can be verified by any auditor without relying on Surveyor's storage infrastructure or the operator's claims.

### **1.2 v0.1 Scope**

v0.1 establishes the foundational evidence collection and trust layer. It does not include the backend API, LLM-powered control evaluation, or gap analysis features. Those layers are only defensible once the evidence foundation is solid.

v0.1 ships:

* Standalone agent binary with scheduler and probe lifecycle management  
* gRPC-based probe plugin framework (go-plugin \+ protobuf)  
* Go SDK for probe authors  
* CycloneDX 1.6 declaration serialization  
* Cosign/Sigstore signing (keyless OIDC \+ BYOK)  
* S3-compatible object storage with Object Lock (WORM)  
* Structured audit event and operational log emission via OpenTelemetry  
* Reference probe: AWS VPC default security group ingress validation  
* Evidence type → control framework mapping (defaults baked into binary, operator overrides via config)

## **2\. Architecture**

### **2.1 Components**

Surveyor v0.1 consists of three primary components: the agent, the probe SDK, and the reference probe. A backend API and evaluation layer are deferred to v0.2.

| Component | Responsibility |
| ----- | ----- |
| Agent | Probe lifecycle, scheduling, signing, storage, OTel emission |
| Probe SDK | Proto definition, Go SDK, Envelope struct |
| Reference Probe | AWS VPC default security group ingress validation |

### **2.2 Deployment Model**

The agent is a standalone Go binary. It reads a YAML configuration file at startup and requires no runtime connection to a central configuration service. All configuration — signing keys, storage endpoints, OTel endpoints, probe definitions — is present on disk before the agent starts.

This design is intentional. Operators deploy the agent via cloud-init, Ansible, Terraform, or any existing provisioning toolchain. The agent starts cold with full knowledge of what to do and how to do it. There is no TOCTOU risk from a remote configuration endpoint.

A future deployment mode using the OpenTelemetry Collector with OpAMP for remote agent management is planned but not included in v0.1. The core library shared between the standalone binary and a future OTel collector plugin is the same.

### **2.3 Trust Chain**

The full verification chain for a piece of evidence is:

1. Probe collects system state snapshot  
2. Agent wraps snapshot in a signed CycloneDX declaration (the Envelope)  
3. Cosign signs the declaration; signature is recorded in Rekor transparency log  
4. Declaration is written to S3 with Object Lock (WORM) enabled  
5. Agent emits an OTel audit event referencing the declaration by content hash and Rekor inclusion proof

An auditor can start from a SIEM event, retrieve the declaration from object storage, verify the content hash, verify the Cosign signature, and confirm the Rekor inclusion proof — all without trusting Surveyor's infrastructure or the operator's assertions.

**BYOK limitation (v0.1):** In BYOK mode without a private Rekor instance, the transparency log inclusion proof is absent. The trust chain degrades to: content hash → Cosign signature → WORM storage. The signature still provides tamper detection and the WORM store provides tamper prevention, but independent transparency log verification is not available. Operators requiring full trust chain integrity in air-gapped environments should deploy a private Rekor instance (documented but not shipped as a Helm chart in v0.1).

## **3\. Agent**

### **3.1 Responsibilities**

* Load and validate YAML configuration at startup; **fail hard on any invalid configuration** (see §7.1)  
* Interrogate probe metadata via Metadata RPC at startup  
* Schedule probe collection runs at randomized intervals  
* Manage probe subprocess lifecycle via go-plugin, including crash recovery (see §3.5)  
* Dispatch collection requests (streaming vs. single envelope) based on probe capability  
* Sign envelopes via Cosign  
* Write declarations to S3-compatible object storage  
* Capture probe logs (hclog/stdlib) and emit as OTel log records  
* Emit audit events and operational telemetry via OTel

### **3.2 Scheduling**

Evidence collection runs at randomized intervals within a configured window. Randomization is intentional: predictable collection schedules create an adversarial surface where a sophisticated actor could temporarily restore compliant state only during known collection windows.

The scheduler configuration specifies a minimum and maximum interval per probe. The agent selects a random interval within that window for each collection run, independently per probe.

### **3.3 Probe Lifecycle (go-plugin)**

Probes are standalone executables that speak the Surveyor probe gRPC protocol. The agent manages probe subprocesses using HashiCorp's go-plugin library (MPL 2.0). go-plugin handles subprocess launch, health checking, gRPC transport, and magic cookie validation (preventing accidental direct invocation of probe binaries).

At agent startup, each configured probe binary is launched and interrogated via the Metadata RPC. The agent records probe ID, version, evidence type, config SHA, and streaming capability. The probe subprocess is then kept alive for the duration of the agent process.

### **3.4 OTel Emission**

The agent is the sole OTel emission point. Probes do not emit OTel directly. The agent captures three categories of signals:

* **Audit events:** evidence collected, declaration signed, declaration stored, signing failures, **collection errors** (probe-reported failures with status ERROR)  
* **Agent operational logs:** scheduler activity, probe lifecycle events, configuration load, errors, **probe crash/restart events**  
* **Probe logs:** captured from probe subprocess via go-plugin's hclog bridge, enriched with probe metadata

All three categories are emitted as OTel log records to a configured OTel endpoint. Operators can route these to any SIEM that accepts OTel. Audit events include the declaration content hash and Rekor inclusion proof, cryptographically linking the SIEM record to the stored artifact.

### **3.5 Probe Crash Recovery**

When the agent detects a probe subprocess has crashed (via go-plugin health check failure or unexpected process exit):

1. Emit an OTel operational log event with probe ID, exit code (if available), and timestamp.  
2. Attempt restart with exponential backoff: 1s, 2s, 4s, 8s, 16s, capped at 60s.  
3. On successful restart, re-interrogate Metadata RPC and resume scheduling.  
4. After 5 consecutive restart failures, mark the probe as **degraded**: stop restart attempts, emit an OTel alert-level event, and skip the probe's scheduled runs. The agent continues operating all other probes.  
5. A degraded probe can be recovered by restarting the agent.

The agent never crashes due to a single probe failure. Probe isolation is a core design guarantee.

### **3.6 Graceful Shutdown**

On SIGTERM or SIGINT:

1. Stop the scheduler (no new collection runs dispatched).  
2. Wait for any in-flight collection runs to complete, sign, and store. Timeout: 30 seconds (configurable via `agent.shutdown_timeout_seconds`).  
3. If in-flight runs do not complete within the timeout, log a warning with details of the abandoned runs and exit.  
4. Terminate probe subprocesses.

Evidence is either fully committed (collected, signed, stored) or not recorded at all. There is no partially-signed or partially-stored state.

## **4\. Probe SDK**

### **4.1 Proto Definition**

The proto definition is the public API contract for the Surveyor probe ecosystem. Probe authors implement this interface; the agent consumes it. Stability matters here above all else.

syntax \= "proto3";

package surveyor.probe.v1;

import "google/protobuf/struct.proto";

import "google/protobuf/timestamp.proto";

enum CollectStatus {

  COLLECT\_STATUS\_SUCCESS \= 0;

  COLLECT\_STATUS\_ERROR   \= 1;

}

message ProbeMetadata {

  string id                 \= 1;

  string version            \= 2;

  string evidence\_type      \= 3;

  string config\_sha         \= 4;

  bool   supports\_streaming \= 5;

}

message CollectRequest {

  string                    probe\_id           \= 1;

  google.protobuf.Struct    config             \= 2;

  google.protobuf.Timestamp scheduled\_at       \= 3;

  string                    collection\_trigger \= 4; // "scheduled" | "manual"

}

message Envelope {

  ProbeMetadata             probe              \= 1;

  string                    host               \= 2;

  string                    ip\_address         \= 3;

  string                    platform           \= 4;

  google.protobuf.Timestamp collected\_at       \= 5;

  string                    content\_type       \= 6;

  bytes                     payload            \= 7;

  string                    collection\_trigger \= 8;

  CollectStatus             status             \= 9;

  string                    error\_detail       \= 10;

}

message CollectResponse  { Envelope envelope \= 1; }

message MetadataRequest  {}

service ProbeService {

  rpc Collect(CollectRequest)       returns (CollectResponse);

  rpc CollectStream(CollectRequest) returns (stream CollectResponse);

  rpc Metadata(MetadataRequest)     returns (ProbeMetadata);

}

### **4.2 Envelope**

The Envelope is the canonical evidence container. Every piece of evidence regardless of probe type carries the same envelope fields. The payload is opaque to Surveyor core — its interpretation is determined by the content\_type field (standard MIME types: application/json, image/png, text/plain, etc.).

This design deliberately avoids a schema registry for probe payloads. Requiring structured schemas would exclude entire categories of valid compliance evidence — screenshots, log excerpts, human-readable configurations — that do not reduce to JSON schemas. The content\_type field provides sufficient type information for downstream consumers (the LLM evaluator in v0.2, human auditors) to interpret payloads correctly.

#### **4.2.1 Partial Failure Semantics**

Probes report success or failure at the individual envelope level using the `CollectStatus` enum:

* **SUCCESS**: The probe collected evidence for this resource. Payload contains the evidence artifact.  
* **ERROR**: The probe attempted to collect evidence for this resource but failed. Payload is empty. `error_detail` contains a human-readable explanation (e.g., "timeout after 30s scanning us-west-2", "access denied for role arn:aws:iam::123:role/x").

Error envelopes are signed and stored identically to success envelopes. A WORM-stored error envelope is proof that collection was attempted — this is compliance-relevant evidence. An auditor can distinguish between "we checked and it was compliant," "we checked and it failed," and "we didn't check" (absence of any envelope).

The agent does not interpret envelope status. It signs and stores whatever the probe emits. Status-based alerting and gap detection are downstream concerns (OTel routing rules, v0.2 backend API).

### **4.3 Streaming vs. Single Envelope**

Probes declare streaming capability in their ProbeMetadata. The agent dispatches to CollectStream for probes that support it, and to Collect for those that do not.

Streaming is appropriate when a single collection run naturally produces multiple independent evidence artifacts — for example, a probe iterating over all VPCs in an AWS account produces one envelope per VPC. Each envelope is independently signed and stored. Probes that encounter errors for some resources can stream success envelopes for resources that succeeded and error envelopes for resources that failed, all within the same collection run.

### **4.4 Go SDK**

The Go SDK wraps the proto definition with ergonomic helpers for probe authors. Probe authors implement a simple interface; the SDK handles gRPC server boilerplate, go-plugin registration, and structured logging via hclog.

Probe authors are infrastructure engineers who know their evidence domain — firewalls, IAM policies, TLS certificates — not necessarily gRPC or compliance frameworks. The SDK should make writing a probe feel like writing a Go function, not implementing a distributed systems protocol.

The SDK provides helper functions for constructing error envelopes, so probe authors don't need to manually populate status and error\_detail fields.

## **5\. Evidence Model**

### **5.1 Evidence Types and Control Mappings**

Evidence types are semantic labels that describe what a probe collects, independent of the specific probe implementation. The agent ships with default mappings from evidence types to compliance framework controls. These defaults represent the standard interpretation of framework requirements.

Operators can override or extend the default mappings via the agent configuration file (see §7). This accommodates organizations whose system boundary definitions or control interpretations differ from the defaults — for example, mapping `network-boundary-controls` to CA-3 in addition to the default SC-7, AC-17, CM-7.

The override mechanism is additive by default: operator-specified mappings are merged with the defaults. Operators can also replace the defaults entirely for a given evidence type by setting `replace: true`.

| Evidence Type | Example Probe | Default Controls |
| ----- | ----- | ----- |
| network-boundary-controls | aws-vpc-default-sg | SC-7, AC-17, CM-7 |
| iam-access-controls | aws-iam-policy-audit | AC-2, AC-3, AC-6 |
| encryption-at-rest | aws-s3-encryption-audit | SC-28 |
| audit-logging | aws-cloudtrail-status | AU-2, AU-3, AU-12 |

The above are illustrative examples. The v0.1 binary ships with default mappings sufficient to support the reference probe.

### **5.2 CycloneDX Serialization**

Evidence declarations are serialized as CycloneDX 1.6 declarations. CycloneDX's declarations specification provides native support for attestations, evidence, and cryptographic signatures — making it a natural fit for Surveyor's evidence model without requiring a custom format.

The internal Envelope struct is Surveyor's canonical representation. CycloneDX is the serialization format. This separation means a v0.2 migration to CycloneDX 2.0 (currently in development, targeting August 2026\) requires a new serializer, not a data migration.

### **5.3 Signing**

Declarations are signed using Cosign with Sigstore. Two signing modes are supported:

* **Keyless (OIDC):** For cloud-native deployments with workload identity (AWS IRSA, GCP Workload Identity, Azure Managed Identity). Short-lived credentials are issued by the OIDC provider; no long-lived key material on the host.  
* **BYOK (Bring Your Own Key):** For bare metal deployments, air-gapped environments, or regulated environments that cannot send signature metadata to the public Rekor transparency log. Key material is configured via the agent config file or environment variables.

Private Rekor instances are supported for environments that require an internal transparency log (e.g., FedRAMP High, DoD IL4/IL5). Deploying a private Rekor instance is documented as a first-class deployment option.

**Note:** BYOK mode without a private Rekor instance does not produce a transparency log inclusion proof. See §2.3 for the impact on the trust chain.

### **5.4 Storage**

Signed declarations are written to S3-compatible object storage with Object Lock enabled in compliance mode (WORM — Write Once Read Many). S3 Object Lock in compliance mode prevents modification or deletion of objects for the configured retention period, even by users with administrative permissions.

The combination of cryptographic signatures (tamper detection) and Object Lock (tamper prevention) provides defense in depth for evidence integrity.

#### **5.4.1 S3 Key Scheme**

Objects are stored with the following key structure:

{probe\_id}/{date}/{host}/{timestamp}-{content\_hash}.cdx.json

Where:

* `probe_id`: the probe's declared ID (e.g., `aws-vpc-default-sg`)  
* `date`: ISO 8601 date of collection (e.g., `2026-04-02`)  
* `host`: the hostname of the collecting agent  
* `timestamp`: Unix epoch seconds of collection time  
* `content_hash`: first 12 characters of the SHA-256 hash of the signed declaration

Example: `aws-vpc-default-sg/2026-04-02/prod-collector-01/1743580800-a1b2c3d4e5f6.cdx.json`

This scheme provides reasonable partitioning by probe, date, and host for the v0.2 query API, supports S3 lifecycle policies by date prefix, and avoids hot-partition issues from high-cardinality prefixes.

## **6\. Reference Probe: AWS VPC Default Security Group**

### **6.1 Purpose**

The reference probe validates that default security groups in AWS VPCs do not allow inbound traffic from the public internet (0.0.0.0/0 or ::/0). This probe is included in v0.1 to:

* Prove the probe plugin framework end-to-end  
* Exercise the CollectStream RPC (one envelope per VPC)  
* Exercise partial failure semantics (success/error per region or VPC)  
* Provide a real, testable compliance control (maps to SC-7, AC-17, CM-7 in NIST 800-53)  
* Serve as the canonical example for probe authors

### **6.2 Collection Logic**

* AWS SDK call to list all VPCs in the configured region(s)  
* For each VPC, retrieve the default security group  
* Inspect all ingress rules for source CIDR 0.0.0.0/0 or ::/0  
* Produce one Envelope per VPC with security group rules as JSON payload (application/json) and status SUCCESS  
* If a region times out or returns an access error, produce one Envelope for that region with status ERROR and a descriptive error\_detail  
* Emit via CollectStream

### **6.3 Configuration**

The probe accepts the following configuration fields via google.protobuf.Struct:

| Field | Description |
| ----- | ----- |
| regions | List of AWS regions to scan (default: current region) |
| role\_arn | Optional IAM role ARN to assume before scanning |
| timeout\_seconds | Per-region scan timeout (default: 30\) |

## **7\. Configuration**

The agent is configured via a single YAML file. All configuration is present on disk before the agent starts; no runtime config fetching is performed.

### **7.1 Config Validation**

The agent validates the entire configuration file at startup. **Any validation error causes the agent to exit immediately with a non-zero exit code and a descriptive error message.** There is no partial startup, no warn-and-skip, no default fallback for required fields.

This is a deliberate design choice: a compliance evidence agent that silently degrades is worse than one that refuses to start. Operators should discover configuration problems at deploy time, not during an audit.

Validated fields include: storage endpoint reachability, signing mode prerequisites (OIDC provider availability for keyless, key file existence for BYOK), probe binary existence and executability, schedule interval validity (min \< max, both \> 0), OTel endpoint format.

### **7.2 Example Configuration**

agent:

  log\_level: info

  otel\_endpoint: "http://localhost:4317"

  shutdown\_timeout\_seconds: 30  \# graceful shutdown timeout

storage:

  type: s3

  bucket: surveyor-evidence

  region: us-east-1

  object\_lock: true

signing:

  mode: keyless          \# or: byok

  \# byok only:

  \# key\_path: /etc/surveyor/cosign.key

  \# rekor\_url: https://rekor.internal.example.com

\# Optional: override or extend default evidence type → control mappings

control\_mappings:

  network-boundary-controls:

    add: \[CA-3\]          \# merge with defaults (SC-7, AC-17, CM-7)

  \# encryption-at-rest:

  \#   replace: true

  \#   controls: \[SC-28, SC-13\]  \# replace defaults entirely

probes:

  \- id: aws-vpc-default-sg

    binary: /usr/local/bin/surveyor-probe-aws-vpc-default-sg

    schedule:

      min\_interval: 3600   \# seconds

      max\_interval: 7200

    config:

      regions: \[us-east-1, us-west-2\]

      timeout\_seconds: 30

## **8\. Repository Structure**

surveyor/

├── proto/

│   └── surveyor/probe/v1/probe.proto

├── sdk/

│   └── go/                  \# Go SDK for probe authors

├── agent/

│   ├── main.go

│   ├── config/              \# YAML loading and validation

│   ├── scheduler/

│   ├── runner/              \# go-plugin probe lifecycle \+ crash recovery

│   ├── signing/             \# Cosign integration

│   ├── storage/             \# S3 client \+ key scheme

│   ├── telemetry/           \# OTel emission

│   └── framework/           \# Evidence type → control mappings (defaults \+ overrides)

├── probes/

│   └── aws-vpc-default-sg/

│       └── main.go

├── test/

│   └── integration/         \# LocalStack-based integration tests

└── docs/

## **9\. Testing Strategy**

### **9.1 Unit Tests**

Standard Go unit tests for all agent packages. Signing and storage interfaces are mockable for unit testing.

### **9.2 Integration Tests**

Integration tests for the AWS VPC reference probe run against LocalStack. These tests validate:

* Probe collects evidence from VPCs created in LocalStack  
* Streaming produces one envelope per VPC  
* Error envelopes are produced for inaccessible regions or permission failures  
* Signed declarations are written to S3 with the correct key scheme  
* Object Lock is applied (where LocalStack supports it)  
* OTel events are emitted for success and error envelopes

LocalStack is run via Docker Compose as part of the test harness. Tests are runnable via `go test ./test/integration/...` with a `-localstack` build tag.

## **10\. Out of Scope for v0.1**

| Feature | Notes |
| ----- | ----- |
| Backend API | Query interface for evidence metadata and artifact retrieval. v0.2. |
| LLM Evaluation | Control satisfaction assessment and gap analysis. Requires backend API. v0.3+. |
| OTel Collector Plugin | go-plugin-based probe loading via OTel collector with OpAMP support. Future. |
| Probe Attestation | Agent-side verification of probe binary integrity before invocation. Future. |
| CycloneDX 2.0 | Targeting August 2026\. Will be a serializer update when spec stabilizes. |
| Schema Registry | Optional payload schema validation for structured probe outputs. Future. |
| Private Rekor Helm Chart | Documented deployment option but not shipped as a chart in v0.1. |
| COLLECT\_STATUS\_PARTIAL | Deferred. Per-resource SUCCESS/ERROR granularity is sufficient for v0.1. If a future probe needs to report mixed results within a single envelope, this can be added. |

## **11\. Key Design Decisions**

### **go-plugin for Probe Distribution**

Probes are standalone executables communicating over gRPC via HashiCorp's go-plugin library. This approach was chosen over Go's native plugin system (Linux-only, fragile) and compiled-in probes (language-locked, requires agent rebuild). go-plugin enables language-agnostic probe authors, subprocess isolation, and independent probe versioning. The protobuf service definition is the public contract.

### **Default Control Mappings with Operator Overrides**

The evidence type → control mapping ships as compiled-in defaults, representing the standard interpretation of framework requirements. Operators can override or extend these mappings via the agent configuration file to accommodate organization-specific control scoping (e.g., adding CA-3 for interconnection agreements). Defaults are updated via agent version upgrades; overrides persist across upgrades.

### **Freeform Payloads with MIME Content Types**

Probe payloads are opaque bytes with a declared MIME content type rather than typed against a schema registry. This accommodates the full range of compliance evidence — structured JSON, screenshots, log excerpts, configuration dumps — without requiring probe authors to register schemas. The LLM evaluator (v0.3+) will dispatch based on content type.

### **Standalone Binary First**

The OTel Collector with OpAMP is compelling for managed deployment scenarios (remote config, agent upgrades) but imposes an operational dependency that bare metal and air-gapped environments cannot satisfy. The standalone binary works everywhere. OTel Collector support is additive via a shared core library.

### **S3 Object Lock over Custom Storage**

S3 Object Lock in compliance mode provides WORM semantics recognized by auditors, without requiring Surveyor to build or operate a custom append-only store. The combination of Cosign signatures (tamper detection) and Object Lock (tamper prevention) provides defense in depth. Any S3-compatible storage (MinIO, Wasabi, etc.) is supported.

### **Fail Hard on Invalid Configuration**

The agent exits immediately on any configuration error at startup. A compliance evidence agent that silently degrades — skipping probes, falling back to defaults for required fields — creates a false sense of coverage. Operators should discover configuration problems at deploy time, not during an audit. This is consistent with the platform's core thesis: verifiable proof over comfortable assumptions.

### **Error Envelopes as Evidence**

Failed collection attempts are signed and stored as first-class evidence. This ensures the audit record distinguishes between "checked and compliant," "checked and failed to collect," and "never checked." The absence of evidence is not evidence of absence — but a stored error envelope is evidence of a coverage gap.

