# V12 编辑页焦点与留白

Mode: Operate. Refine the existing editor in response to the supplied screenshot; retain paper/ink/blue, sidebar, actual content, controls and all V11 save/recovery behavior.

## Direction contract

THESIS: A continuous writing page with compact entry controls and a quiet, visible focus state.

OWN-WORLD: Existing tokens and typography; no new artwork, dependencies or behavior. Keep readable prose and all 44px controls.

STORY: Task and save state → title → editing mode / formatting → prose; article metadata remains supporting information.

FIRST VIEWPORT: Remove the inspector-induced title gap, tighten title/mode/toolbar vertical space, and replace the full-height blue focus rectangle with the existing toolbar divider in accent. Active modes use a small underline rather than a blue tile.

FORM: Desktop first grid row is intrinsic and the size-contained, independently scrolling inspector follows the writing height; title/body retain the same max850px box and paired32/28/18px horizontal insets. Mobile keeps one disclosure and wrapping tools. Title has an accent underline on focus; document focus uses its toolbar divider; command keyboard outlines remain. Preserve selection/caret colors and forced-color focus.

FINISH: Verify actual empty focused and populated selection states at desktop, tall desktop, tablet and mobile in both themes. Run focused layout and V11 safety checks, one scoped detector, inspect captures and obtain independent finish review; update design evidence narrowly.
