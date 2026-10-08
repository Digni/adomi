## Purpose

Provide traceable local copies of work-item attachments and embedded images so agents can inspect the evidence associated with requirements and discussion.

## ADDED Requirements

### Requirement: Discover image references within exported work-item scope
The system SHALL discover HTML img src references in string-valued fields of every exported work item, including description, repro steps, acceptance criteria, and custom fields. With --include-comments it SHALL discover HTML and Markdown images in included comments according to their format; missing or unknown format SHALL support Markdown and embedded HTML. Markdown image references inside code examples SHALL NOT cause downloads. Ordinary hyperlinks, CSS images, and srcset SHALL NOT be treated as image references. Item traversal SHALL remain unchanged and original item/comment JSON and escaped HTML SHALL be preserved.

#### Scenario: Images appear outside description
- **WHEN** a parent, root, or direct-child item has an img src in repro steps, acceptance criteria, or a custom string field
- **THEN** that reference is represented with its field name in the item's asset manifest

#### Scenario: Markdown discussion has image syntax and code examples
- **WHEN** an included Markdown comment has inline or reference-style images alongside fenced or inline code showing image syntax
- **THEN** real image references are discovered with comment provenance and code examples are ignored

### Requirement: Limit authenticated inline retrieval to configured attachment resources
Inline references SHALL be resolved from configured Azure DevOps base URL and project context, never from content-supplied base elements or author URLs. Supported resources SHALL be work-item attachment endpoints on the configured scheme and host within the configured organization or collection. Absolute, root-relative, and project-relative supported endpoints SHALL be handled. Userinfo, unsupported schemes, unsafe encoded or traversal paths, other origins and other endpoints SHALL be skipped with a reason and without any network request. Existing redirect rejection, timeout, and 64 MiB per-file limit SHALL remain enforced.

#### Scenario: External or unsafe image reference
- **WHEN** content refers to another host, another organization, a data URL, or a non-attachment endpoint
- **THEN** the manifest marks it skipped with a reason and no request or credential transmission occurs for that reference

#### Scenario: Relative attachment reference
- **WHEN** a supported image src uses a root-relative or project-relative attachment URL
- **THEN** it resolves under configured context and is downloaded through the existing authenticated client

### Requirement: Deduplicate assets and preserve source provenance
Within each work item the system SHALL download a resource once across attachment relations, fields and included comments. Supported attachment identity SHALL ignore fragment and presentation-only fileName, download and api-version query parameters, while preserving other query parameters. Every distinct attachment, field or comment source SHALL remain associated with the resulting asset. Deduplication SHALL NOT relocate files between work items.

#### Scenario: Same screenshot is attached and embedded
- **WHEN** an attachment relation and two fields or comments reference the same attachment with presentation query differences
- **THEN** one file is written for that item, the download count increases once, and all sources appear in its manifest entry

### Requirement: Export addressable asset manifests
Every successful work-item export SHALL include assets/<item-id>.json and an additive index assetsPath for each item. The manifest SHALL contain workItemId and a non-null assets array. Asset entries SHALL include original url, status, and sources; downloaded entries SHALL include name and an export-relative POSIX path; skipped entries SHALL include a reason and no local path. Source entries SHALL identify kind (attachment, field, comment), the original reference URL, and field name or comment ID when applicable. A metadata-only export without a downloader SHALL label undispatched references skipped. Refreshes SHALL remove stale image files and source references through the existing replacement behavior.

#### Scenario: No image evidence exists
- **WHEN** an item has no attachments or discovered image references
- **THEN** its manifest contains an empty assets array and creates no attachment files

#### Scenario: Comments omitted on refresh
- **WHEN** an export with comment-only images is refreshed without --include-comments
- **THEN** those comment sources and files are absent from the new export unless also referenced by included content

### Requirement: Preserve downloadable evidence without filename collisions
Downloaded bytes SHALL be preserved. Filenames SHALL use relation names when available, then fileName query values, then URL basenames, with safe fallback names and existing length/path protections. Inline image files SHALL receive an extension matching a recognized image type when their supplied extension is missing or misleading. All final names SHALL be unique case-insensitively, including names generated to resolve earlier collisions. Ordinary attachments SHALL continue supporting non-image data.

#### Scenario: Generated filename already exists
- **WHEN** assets have names same.png, same.png and same-2.png, or case-only name differences
- **THEN** every asset retains its own bytes at a distinct safe local path

#### Scenario: Image URL ends in an opaque identifier
- **WHEN** a downloaded inline image has no useful filename or extension
- **THEN** its local filename has an extension matching the recognized image bytes

### Requirement: Supported download failures cannot masquerade as complete evidence
A supported resource download failure, invalid inline image body, or file-write failure SHALL fail the export, leave success stdout empty, and prevent final metadata publication. Files already written in the failed attempt MAY remain as under current attachment failure behavior. Unsupported inline references SHALL instead be explicitly skipped. Only physical file writes SHALL emit exporter download progress or increment attachment counts.

#### Scenario: Inline image endpoint returns HTML
- **WHEN** a supported inline image download returns a successful HTTP status with a non-image body
- **THEN** the export fails rather than saving that body as a successfully downloaded screenshot

#### Scenario: Skipped source is not counted as downloaded
- **WHEN** an export has one downloaded screenshot and one unsupported external reference
- **THEN** its attachment count is one and the manifest distinguishes downloaded and skipped entries
