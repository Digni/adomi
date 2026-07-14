# ado-wiki-context Specification

## Purpose
Define read-only Azure DevOps wiki page retrieval and deterministic repository-local context exports.

## Requirements

### Requirement: Resolve an Azure DevOps wiki
The system SHALL resolve the user-supplied wiki ID or name within the configured Azure DevOps project before fetching page content and SHALL use the canonical wiki ID returned by Azure DevOps as the export identity.

#### Scenario: Resolve wiki by ID or name
- **WHEN** the user requests wiki context with a non-blank wiki ID or name and Azure DevOps returns a wiki with a non-blank canonical ID
- **THEN** the system uses that wiki metadata and canonical ID for page requests and the local export bundle

#### Scenario: Wiki cannot be resolved
- **WHEN** Azure DevOps returns an HTTP error, authorization redirect, network error, malformed JSON, empty response, or wiki metadata without a canonical ID
- **THEN** the fetch fails without requesting page content and without printing success data to stdout

### Requirement: Fetch one wiki page or a recursive page subtree
The system SHALL fetch Markdown content for the requested absolute wiki page path and SHALL fetch all descendant pages only when recursive selection is explicitly requested.

#### Scenario: Fetch one page
- **WHEN** the user requests a valid absolute page path without recursive selection
- **THEN** the system fetches that page with content and does not fetch descendant page content

#### Scenario: Fetch a recursive subtree
- **WHEN** the user requests a valid absolute page path with recursive selection
- **THEN** the system obtains the complete descendant metadata tree and fetches content for the requested page and every unique descendant page

#### Scenario: Fetch the recursive wiki root
- **WHEN** the user requests `/` with recursive selection
- **THEN** every absolute page path in the returned root tree is treated as a descendant and fetched

#### Scenario: Empty page content
- **WHEN** Azure DevOps returns a valid requested page whose Markdown content is empty
- **THEN** the system treats the page as valid and exports an empty Markdown file with its page metadata

#### Scenario: Page fetch fails
- **WHEN** any requested page or descendant returns an HTTP error, authorization redirect, network error, malformed JSON, empty response, metadata without an absolute page path, or a page path different from the requested path
- **THEN** the complete fetch fails and no new export bundle is written

#### Scenario: Recursive response contains duplicate page paths
- **WHEN** Azure DevOps returns the same page path more than once in a recursive metadata tree
- **THEN** the fetch fails rather than silently overwriting or duplicating a page in the export

#### Scenario: Recursive response contains a page outside the requested subtree
- **WHEN** Azure DevOps returns an absolute metadata path that is neither the requested page nor a descendant beginning with the requested page path plus `/`
- **THEN** the fetch fails without requesting or exporting content for that path

### Requirement: Export deterministic repository-local wiki context
The system SHALL export successfully fetched wiki context under `.adomi/context/wikis/<canonical-wiki-id>/` with the resolved wiki metadata, fetched page metadata, Markdown files, and an index that maps each original wiki path to its relative Markdown file path.

#### Scenario: Export a single page bundle
- **WHEN** one wiki page is fetched successfully
- **THEN** the bundle contains `wiki.json`, `pages.json`, `index.json`, and one Markdown file below `pages/`, and the index identifies the profile, project, wiki, requested page path, non-recursive selection, creation time, and Markdown file mapping

#### Scenario: Export a recursive bundle deterministically
- **WHEN** a wiki page subtree is fetched successfully
- **THEN** page metadata and index entries are ordered by original wiki path and every page maps to one distinct Markdown file below `pages/`

#### Scenario: Unsafe or colliding wiki paths
- **WHEN** page paths contain traversal-like, platform-unsafe, or colliding filesystem representations
- **THEN** the exporter either maps every original path to a distinct safe relative path contained below the bundle's `pages/` directory or fails before overwriting another page or writing outside the bundle

#### Scenario: Export destination contains a symbolic link
- **WHEN** an existing path component below the repository root in `.adomi/context/wikis/<canonical-wiki-id>` is a symbolic link
- **THEN** the exporter fails before removing or writing files through that link

#### Scenario: Successful refetch replaces stale context
- **WHEN** a later fetch of the same canonical wiki ID succeeds with a different page selection or page set
- **THEN** the new bundle replaces the previous bundle and does not retain stale page or metadata files

#### Scenario: Export write fails
- **WHEN** directory creation, JSON encoding, or file writing fails during export
- **THEN** the command returns an error, writes no success data to stdout, and does not write a final success `index.json`

### Requirement: Wiki context fetch remains read-only in scope
The system SHALL use only Azure DevOps wiki read operations for this capability and SHALL NOT perform indexed wiki search, wiki mutation, version override, wiki attachment mirroring, or archival reconstruction.

#### Scenario: Fetch does not invoke excluded capabilities
- **WHEN** wiki context is fetched successfully
- **THEN** the system resolves the wiki and reads page metadata/content without calling Azure DevOps Search or any wiki create, update, or delete endpoint

#### Scenario: Attachments remain referenced content
- **WHEN** fetched Markdown contains links to Azure DevOps wiki attachments or other remote resources
- **THEN** the Markdown links remain in the exported content and the referenced resources are not downloaded into the bundle
