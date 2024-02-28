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
  const addedATEFiles = new Set();
  const changedATEFiles = new Set();
  const changedOTGFiles = new Set();
  const rspListFiles = await github.paginate(listFiles);
  rspListFiles.forEach(f =>  {
    switch(f.status) {
      case "added":
      case "copied":
        if (f.filename.includes('ate_tests')) {
          addedATEFiles.add(f.filename);
        }
        break;
      case "modified":
        if (f.filename.includes('ate_tests')) {
          changedATEFiles.add(f.filename);
        } else if (f.filename.includes('otg_tests')) {
          changedOTGFiles.add(f.filename);
        }
        break;
    }
  });
  if (addedATEFiles.size != 0) {
    core.warning('Cannot add new ATE test files: ' + [...addedATEFiles].join(','));
  }
  changedATEFiles.forEach(file => {
    console.log('Found changed ATE file: ' + file);
    const otgFile = file.replace('ate_tests', 'otg_tests');
    if (fpFiles.has(otgFile)) {
      console.log('Found matching OTG file: ' + otgFile);
      if (!changedOTGFiles.has(otgFile)) {
        core.setFailed(otgFile + ' needs to be updated to match ' + file)
      }
    }
  })
}
