# [validate-codeowners](https://gitlab.com/tedspinks/validate-codeowners)

This CI/CD component provides a hidden job that you can extend to validate your GitLab project's CODEOWNERS file. It also includes a Linux binary and Docker image, which you can run from your desktop or from your own CI/CD job.

It performs the following validation checks:

- CODEOWNERS file resides in one of the three [supported locations](https://docs.gitlab.com/ee/user/project/codeowners/#codeowners-file).
- [Syntax](https://docs.gitlab.com/ee/user/project/codeowners/reference.html) is valid.
- All owners are valid GitLab @groups, @users, or user@emails.
- All @groups are **direct** members of the project.
- All @users are **direct** members of the project.
- All user@emails are **direct** members of the project.


## About direct memberships

What's all the fuss about checking [direct](https://docs.gitlab.com/ee/user/project/members/) memberships? From the [GitLab documentation](https://docs.gitlab.com/ee/user/project/codeowners/#group-inheritance-and-eligibility):

> For approval to be required, groups as Code Owners must have a direct membership (not inherited membership) in the project. Approval can only be optional for groups that inherit membership. Members in the Code Owners group also must be direct members, and not inherit membership from any parent groups.

Since we almost always want our CODEOWNERS file to **enforce** specific approvals, this job makes sure that the required direct memberships are present.


## Example CI/CD Component Usage

.gitlab-ci.yml
```yaml
include:
  - project: tedspinks/validate-codeowners
    ref: v1.0.0  # or "main" to always run the latest version
    file: templates/validate-codeowners.yml
    inputs:
      GITLAB_TOKEN: ${GITLAB_TOKEN}

validate-codeowners:
  extends: .validate-codeowners
  stage: test
  # Example rule will only run this job in MRs, when the CODEOWNERS file has changed
  rules:
    - if: $CI_PIPELINE_SOURCE == 'merge_request_event'
      changes:
        - CODEOWNERS
        - docs/CODEOWNERS
        - .gitlab/CODEOWNERS
```


## Example CLI Usage

```bash
cd my-git-clone-directory

export GITLAB_TOKEN=glpat-blahblah12345
export CI_PROJECT_PATH=my-group/my-project-with-codeowners-file
export CI_COMMIT_REF_NAME=my-branch
export CI_API_GRAPHQL_URL=https://gitlab.com/api/graphql
export CI_API_V4_URL=https://gitlab.com/api/v4

./validate-codeowners
```

## Inputs

#### CI/CD Component Inputs

- `GITLAB_TOKEN` - **Required**. GitLab token with "read_api" scope. This is usually an Admin token. See token [permission details](https://docs.gitlab.com/ee/api/members.html). Summary of required permissions:
  1. Owner of the target project.
  2. Member of ALL groups that might be listed as Codeowners (or that might contain users listed as Codeowners).
  3. To validate emails: group owners for enterprise users, or admin for self-hosted.
- `GITLAB_TIMEOUT_SECS` - Optional. Timeout in seconds for communication with the GitLab APIs. Default is "30".

#### Pipeline Variables

- `CODEOWNERS_DEBUG` - Optional. Set to "true" for debug logging (it's VERY verbose). Handy for manual pipeline runs in the web UI.

#### GitLab [Predefined variables](https://docs.gitlab.com/ee/ci/variables/predefined_variables.html)

- `CI_PROJECT_PATH` - The project namespace with the project name included.
- `CI_COMMIT_REF_NAME` - The branch or tag name for which project is built.
- `CI_API_GRAPHQL_URL` - The GitLab API GraphQL root URL
- `CI_API_V4_URL` - The GitLab API v4 root URL.


## Technical Design Considerations

The GitLab GraphQL API includes a nice [CODEOWNERS syntax validator](https://docs.gitlab.com/ee/api/graphql/reference/#repositoryvalidatecodeownerfile). This is the same validator that runs when you edit a CODEWONERS file from the GitLab web UI. Rather than re-invent the wheel and write a complete parser, I decided to make use of this excellent API function. With syntax taken care of, I was able to write a *much simpler* `splitCodeownersLine()` function, which just grabs the file patterns and owners from each line.

To do the actual validations, I tried to use GitLab's newer GraphQL API as mush as possible. However, it wasn't apparent how to get a project's `shared_with_groups` field from the GraphQL queries, so I ended up using the REST `projects/` endpoint for that piece.
