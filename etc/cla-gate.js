// Posts the required `cla/gate` commit status for .github/workflows/cla-gate.yml, through actions/github-script.

// Honored only on same-repo PRs, matching pr-labeler.yml.
const TRUSTED_BOTS = ['dependabot[bot]', 'github-actions[bot]', 'viam-ci-org-reader[bot]', 'viam-overwatch[bot]'];

function memberChecker(orgClient, org) {
  const cache = new Map();
  return (login) => {
    if (!cache.has(login)) {
      cache.set(login, orgClient.rest.orgs.checkMembershipForUser({ org, username: login }).then(
        (res) => res.status === 204,
        (err) => {
          if (err.status === 404) return false;
          throw err;
        },
      ));
    }
    return cache.get(login);
  };
}

async function findPulls(github, context) {
  if (context.payload.pull_request) return [context.payload.pull_request];
  // commits/{sha}/pulls omits fork PRs, so match head SHAs against the open PR list.
  const open = await github.paginate(github.rest.pulls.list, { ...context.repo, state: 'open', base: 'main', per_page: 100 });
  return context.eventName === 'status' ? open.filter((pr) => pr.head.sha === context.payload.sha) : open;
}

async function findOutsiders(github, repo, pr, isMember) {
  const isFork = pr.head.repo?.full_name !== pr.base.repo.full_name;
  const commits = await github.paginate(github.rest.pulls.listCommits, { ...repo, pull_number: pr.number, per_page: 100 });
  // Same identity cla-assistant.io checks: the author's GitHub user, else the committer's.
  const logins = new Set([pr.user.login, ...commits.map((c) => c.author?.login ?? c.committer?.login ?? null)]);
  const outsiders = [];
  for (const login of logins) {
    const trusted = login !== null && ((!isFork && TRUSTED_BOTS.includes(login)) || (await isMember(login)));
    if (!trusted) outsiders.push(login ?? 'unlinked commit author');
  }
  return outsiders;
}

async function gateStatus(github, repo, pr, isMember) {
  const outsiders = await findOutsiders(github, repo, pr, isMember);
  if (outsiders.length === 0) {
    return { state: 'success', description: 'All committers are viamrobotics members' };
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
  const isMember = memberChecker(orgClient, repo.owner);
  for (const pr of await findPulls(github, context)) {
    const status = await gateStatus(github, repo, pr, isMember);
    await github.rest.repos.createCommitStatus({ ...repo, sha: pr.head.sha, context: 'cla/gate', ...status });
    core.info(`#${pr.number} ${pr.head.sha}: ${status.state} (${status.description})`);
  }
};
