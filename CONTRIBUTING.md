# Contributing to Audit-Agent (CRA & DORA)

Thank you for your interest in contributing! We welcome contributions from the community to help improve the security and resilience of our systems.

## Getting Started

1.  **Fork the repository** on GitHub.
2.  **Clone your fork** locally: `git clone https://github.com/your-username/Audit-Agent.git`
3.  **Create a new branch** for your feature or bug fix: `git checkout -b my-new-feature`

## Code Style

*   We follow standard Go coding conventions.
*   Please run `gofmt` on your code before submitting.
*   Ensure variable and function names are descriptive.
*   Add comments for exported functions and complex logic.

## Testing Strategy

*   We use the standard Go testing framework (`testing` package).
*   **Run all tests** before submitting your changes:
    ```bash
    go test ./...
    ```
*   Add new tests for any new functionality you introduce.
*   Ensure that your changes do not break existing tests.

## Submitting Pull Requests

1.  Push your branch to your fork: `git push origin my-new-feature`
2.  Open a **Pull Request** against the `main` branch of the original repository.
3.  Provide a clear description of your changes and why they are necessary.
4.  Link to any relevant issues.
5.  Wait for a code review. We aim to review PRs within a few days.

## Code of Conduct

Please note that we expect all contributors to adhere to our Code of Conduct. Be respectful and constructive in your communications.
