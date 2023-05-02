module.exports = async ({github, context, core}) => {
  // Get all files in the repository in the 'main' branch
  const fpFiles = new Set();
  const rspGetTree = await github.rest.git.getTree({
    owner: context.repo.owner,
    repo: context.repo.repo,
    tree_sha: 'main',
    recursive: true
  });
  rspGetTree.data.tree.forEach(t => fpFiles.add(t.path))
  // Get all changed files in a pull request
  const listFiles = github.rest.pulls.listFiles.endpoint.merge({
    owner: context.repo.owner,
    repo: context.repo.repo,
    pull_number: context.payload.pull_request.number
  });
  const changedFiles = new Set();
  const rspListFiles = await github.paginate(listFiles);
  rspListFiles.forEach(f => changedFiles.add(f.filename));
  changedFiles.forEach(file => {
    console.log('Found changed file: ' + file);
    if (file.includes('ate_tests')) {
      const otgFile = file.replace('ate_tests', 'otg_tests');
      if (fpFiles.has(otgFile)) {
        console.log('Found matching OTG file: ' + otgFile);
        if (!changedFiles.has(otgFile)) {
          core.setFailed(otgFile + ' needs to be updated to match ' + file)
        }
      }
    }
  })
}
