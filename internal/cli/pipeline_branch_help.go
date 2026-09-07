package cli

const adoPipelineBranchHelp = `
Branch filtering:
  list --branch <branch> sends a server-side branchName filter on every request. Short names receive the refs/heads/ prefix; full refs are preserved after trimming whitespace.
  Filtering remains project-scoped and does not infer pull-request validation refs or strip remote-name prefixes. The get command does not accept --branch.
`
