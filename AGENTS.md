# Agent Instructions

## After Each Change

At the end of every change, run the following two scripts in order:

1. **Tests** — run the full test suite with coverage:
   ```
   ./test.sh
   ```

2. **SonarQube analysis** — submit coverage results to SonarQube:
   ```
   ./misc/bin/sonar.sh
   ```

Both scripts must pass before a change is considered complete.
