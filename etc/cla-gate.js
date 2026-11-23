// Posts the required `cla/gate` commit status for .github/workflows/cla-gate.yml, through actions/github-script.

async function isMember(orgClient, org, login) {
  try {
    const res = await orgClient.rest.orgs.checkMembershipForUser({ org, username: login });
    return res.status === 204;
  } catch (err) {
    if (err.status === 404) return false;
    throw err;
  }
}

async function findPulls(github, context) {
  if (context.payload.pull_request) return [context.payload.pull_request];
  // commits/{sha}/pulls omits fork PRs, so match head SHAs against the open PR list.
  const open = await github.paginate(github.rest.pulls.list, { ...context.repo, state: 'open', base: 'main', per_page: 100 });
  return context.eventName === 'status' ? open.filter((pr) => pr.head.sha === context.payload.sha) : open;
}

async function findOutsiders(github, repo, pr, isTrusted) {
  const isFork = pr.head.repo?.full_name !== pr.base.repo.full_name;
  const commits = await github.paginate(github.rest.pulls.listCommits, { ...repo, pull_number: pr.number, per_page: 100 });
  // Same identity cla-assistant.io checks: the author's GitHub user, else the committer's.
  const logins = new Set([pr.user.login, ...commits.map((c) => c.author?.login ?? c.committer?.login ?? null)]);
  const outsiders = [];
  for (const login of logins) {
    const trusted = login !== null && (await isTrusted(login, isFork));
    if (!trusted) outsiders.push(login ?? 'unlinked commit author');
  }
  return outsiders;
}

async function gateStatus(github, repo, pr, isTrusted) {
  const outsiders = await findOutsiders(github, repo, pr, isTrusted);
  if (outsiders.length === 0) {
    return { state: 'success', description: 'All committers are viamrobotics members or trusted bots' };
  }
  const { data } = await github.rest.repos.getCombinedStatusForRef({ ...repo, ref: pr.head.sha, per_page: 100 });
  const cla = data.statuses.find((s) => s.context === 'license/cla');
  return {
    state: cla?.state ?? 'pending',
    description: `license/cla ${cla?.state ?? 'not reported'} for ${outsiders.join(', ')}`.slice(0, 140),
    target_url: cla?.target_url ?? undefined,
  };
}

module.exports = async ({ github, context, core, orgClient }) => {
  const repo = context.repo;
  const dryRun = process.env.DRY_RUN === 'true';
  const trustedBots = (process.env.TRUSTED_BOTS ?? '').split(',').map((s) => s.trim()).filter(Boolean);
  // Bots are honored only on same-repo PRs, matching pr-labeler.yml.
  const isTrusted = (login, isFork) => (!isFork && trustedBots.includes(login)) || isMember(orgClient, repo.owner, login);
  let failures = 0;
  for (const pr of await findPulls(github, context)) {
    try {
      const status = await gateStatus(github, repo, pr, isTrusted);
      if (!dryRun) await github.rest.repos.createCommitStatus({ ...repo, sha: pr.head.sha, context: 'cla/gate', ...status });
      core.info(`${dryRun ? '[dry run] ' : ''}#${pr.number} ${pr.head.sha}: ${status.state} (${status.description})`);
    } catch (err) {
      failures++;
      core.error(`#${pr.number} ${pr.head.sha}: ${err.message}`);
    }
  }
  if (failures > 0) core.setFailed(`${failures} PR(s) got no cla/gate status`);
};
