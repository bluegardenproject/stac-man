package service

import (
	"fmt"
	"strings"

	"github.com/bluegardenproject/stac-man/internal/stack"
)

// Sentinel pair fencing the auto-generated stack table inside a PR
// body. The exact strings must never change once shipped — they are
// the contract that lets re-runs of `sm submit` find and replace
// the previous block instead of accumulating duplicates.
const (
	stackTableStart = "<!-- stac-man:stack-start -->"
	stackTableEnd   = "<!-- stac-man:stack-end -->"
)

// stackChainForBranch returns the linear path of tracked branches
// surrounding `branch`: every ancestor (trunk → branch, oldest first),
// the branch itself, then every descendant in depth-first preorder.
//
// We deliberately limit the chain to this branch's path rather than
// rendering the entire stack graph: a non-linear stack with siblings
// would otherwise produce an awkward tree inside every sibling's PR
// body, where 90% of the entries are unrelated to that PR. The path
// view matches what a reviewer of #N actually wants to know — what's
// below me, what's above me, and where do I sit.
func stackChainForBranch(g *stack.Graph, branch string) []stack.Branch {
	if g == nil {
		return nil
	}
	ancestors := g.Ancestors(branch)
	out := make([]stack.Branch, 0, len(ancestors)+1)
	for i := len(ancestors) - 1; i >= 0; i-- {
		out = append(out, ancestors[i])
	}
	if self, ok := g.Get(branch); ok {
		out = append(out, self)
	}
	out = append(out, g.Descendants(branch)...)
	return out
}

// renderStackTable produces the markdown block that gets injected
// into a PR body. It returns "" for chains that aren't worth
// rendering — a branch sitting alone on trunk gets no decoration
// because there's no "stack" to describe.
//
// `current` is the branch whose PR will receive this table; its
// row is bolded and tagged with "← this PR". `prByBranch` maps
// branch names to their PR numbers; entries without a PR are
// rendered as the bare branch name (so the table still tells the
// reviewer where the unsubmitted gap is).
func renderStackTable(chain []stack.Branch, current string, prByBranch map[string]int) string {
	if len(chain) <= 1 {
		return ""
	}
	var b strings.Builder
	b.WriteString(stackTableStart)
	b.WriteString("\n**Stack** _(managed by [stac-man](https://github.com/bluegardenproject/stac-man))_\n\n")
	for _, branch := range chain {
		line := stackTableLine(branch, current, prByBranch)
		b.WriteString("- ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString(stackTableEnd)
	return b.String()
}

// stackTableLine renders a single bullet inside the stack table.
// Pulled out as a separate function so the per-branch formatting
// (PR link, branch name, current marker) has a single unit-tested
// home and stays consistent across all rows.
func stackTableLine(branch stack.Branch, current string, prByBranch map[string]int) string {
	label := branch.Name
	if pr, ok := prByBranch[branch.Name]; ok && pr > 0 {
		label = fmt.Sprintf("#%d %s", pr, branch.Name)
	} else {
		label = fmt.Sprintf("%s _(no PR)_", branch.Name)
	}
	if branch.Name == current {
		return fmt.Sprintf("**%s ← this PR**", label)
	}
	return label
}

// injectStackTable returns body with table merged in. When a
// previous block fenced by the sentinels exists, its content is
// replaced in-place so manual edits outside the sentinels survive
// every re-run. Otherwise the table is prepended with a blank-line
// separator so the user's original body still reads cleanly below.
//
// Idempotency: injectStackTable(injectStackTable(body, t), t) ==
// injectStackTable(body, t). This is what lets `sm submit` run as
// many times as the user likes without the body growing.
func injectStackTable(body, table string) string {
	if table == "" {
		return body
	}
	startIdx := strings.Index(body, stackTableStart)
	endIdx := strings.Index(body, stackTableEnd)
	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		// Splice the new table in place of the old block, keeping
		// everything before startIdx and everything after the end
		// sentinel. The new `table` already includes both sentinels.
		afterEnd := endIdx + len(stackTableEnd)
		return body[:startIdx] + table + body[afterEnd:]
	}
	if strings.TrimSpace(body) == "" {
		return table
	}
	return table + "\n\n" + body
}
