# Check whether a literal <!here> typed in the web client draws as a mention

## Context

`blockkit.RichTextToMrkdwn` turns a rich_text block into Slack mrkdwn
for the renderer and does not escape its text runs. A text element
whose text is literally `<!here>` or `<@U123>` (typed in the web client
inside a way that keeps it text) would then reach
`RenderSlackMarkdown` looking like a real token and be drawn as a
mention.

This came out of the edit-as-typed port (upstream #395): the edit seed
escapes such text (`blockkit.EscapeLiteralTokens`), and the display
path was noticed to lack the same care. It is untested.

## Acceptance criteria

- [ ] A test renders a block whose text element is a literal token and shows what is drawn.
- [ ] If it draws as a mention, `RichTextToMrkdwn` entity-escapes text runs (the renderers already decode `&lt;` `&gt;` `&amp;`), with a property over `proptest` text that a text element never yields a token.

## Plan

## Log
