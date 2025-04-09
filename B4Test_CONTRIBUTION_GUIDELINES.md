# PR Review Process and Coding Best Practices

This document outlines the PR review process and coding best practices to ensure high-quality contributions and foster a
collaborative coding culture.

# Goals

- Improve compliance with the PR review process.
- Enhance code quality, maintainability, and team collaboration.

---

# PR Review Process

### 1. Smaller, Incremental PRs

- Break down large features into smaller, self-contained PRs (Minimum Viable Product - MVP).
- Benefits:
    - Early feedback.# PR Review Process and Coding Best Practices

  This document outlines the PR review process and coding best practices to ensure high-quality contributions and foster
  a collaborative coding culture.
    
  ---

  ## Goals

    - Improve compliance with the PR review process.
    - Enhance code quality, maintainability, and team collaboration.

    ---

  ## PR Review Process

  ### 1. Smaller, Incremental PRs

    - Break down large features into smaller, self-contained PRs (**Minimum Viable Product - MVP**).
    - Benefits:
        - Early feedback.
        - Easier reviews.
        - Faster iterations.

  ### 2. Standardized Reviewer Roles

    - **Primary Reviewer**: Focus on technical correctness (e.g., gRIBI, gNSI, XR logic).
    - **Secondary Reviewer**: Ensure readability, maintainability, and adherence to best practices.
    - Both reviewers provide overall feedback, but roles ensure structured reviews.

  ### 3. Review Timelines

    - Reviewers: Provide feedback within **2 working days**.
    - PR Authors: Follow up actively and provide ETAs for pending changes.
    - **IMPORTANT**: The responsibility of follow-ups lies with the **PR author** to ensure PR is merged in a timely manner.

    ---

  ## Coding Best Practices

  ### Before Submitting a PR:

    1. Run Sanity Checks:
        - Use tools like GitHub Copilot for:
            - Logic improvements.
            - Refactoring repeated blocks.
            - Spelling and naming consistency.
        - This allows reviewers to focus on the logic and functionality of the code.

    2. Documentation:
        - Add a concise `README` for new features:
            - Feature name, Jira ID, test steps, dependencies, and special configurations.
        - Document caveats, important limitations, and dependencies in comments and/or the `README`.
        - **More documentation is always preferable to less.**

    3. Code Comments:
        - Add comments for non-obvious code blocks to aid future debugging. This helps everyone on the team down the
          line.

  ### General Guidelines:

    - Write **small, focused changes** for easier reviews.
        - say we have a feature X which needs 20 tests spanning 10-15 OC paths. Break down the feature into smaller,
          MVP-focused phases instead of one monolithic diff.
        - Provide **pass logs** for new feature tests over multiple iterations.
    - Use the GitHub "resolve conversation" feature to mark comments as resolved, only after addressing them.
    - Avoid writing Go tests with Python-style naming conventions. For example, use `isReloadSupported` instead of `is_reload_supported` to follow Go's standard naming practices.


    ---

  ## Reviewer Checklist

    - Does this code add technical debt?
    - Is it easy to maintain and understand?
    - Can the team maintain this code without the original author?

    ---

  ## Expectations

    - PR Authors: Ensure timely follow-ups and provide visibility on progress.
    - Reviewers: Balance diligence with timely feedback to avoid delays.

  By adhering to these practices, we can improve the quality of our codebase and foster a collaborative, efficient
  coding culture.
    
  ---
## Important Links

- [GitHub Copilot Overview](https://runon.cisco.com/c/r/runon/Services/devops_github_copilot/ov/github_copilot_overview.html#devops_github_copilot-github-copilot)
- GitHub Copilot [Documentation](https://docs.github.com/en/copilot)
- Software Engineering at Google by O'Reilly Publications (https://learning.oreilly.com/library/view/software-engineering-at/9781492082781/)

  ## TODO
    - April 9th, 2024: Add a table of ownership of different components within B4.