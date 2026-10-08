## ADDED Requirements

### Requirement: Generated skill explains work-item image evidence
The generated skill SHALL direct agents to index assetsPath and assets/<item-id>.json to map downloaded attachments and inline images to fields or comments. It SHALL explain skipped references, require inspecting local image files before claiming to have seen screenshots, and state that comment images require --include-comments. It SHALL distinguish downloaded-file counts from comment-read progress and preserve the raw-content and escaped-HTML explanation.

#### Scenario: Agent inspects a screenshot
- **WHEN** an agent reads the generated work-item guidance
- **THEN** it can locate each downloaded image through the manifest and distinguish a skipped reference from an available local image
