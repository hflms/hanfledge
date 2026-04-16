1.  **Extract Helper Functions**: Use `run_in_bash_session` to add four new helper functions to `internal/delivery/http/handler/dashboard.go`:
    *   `loadKPSequence`: parses the JSON KPIDS from the activity, loads the KP models to get titles, and builds a list of IDs, a sequence list, and an index map.
    *   `loadStudentNames`: given a list of sessions, extracts unique student IDs and loads their display names from the database, returning the list of IDs and a map of ID to name.
    *   `loadStudentMasteryMap`: takes lists of student IDs and KP IDs, queries the mastery repository, and builds a map of `masteryKey` (struct with StudentID, KPID) to mastery score. I will define `masteryKey` struct inside the handler file so it can be shared.
    *   `processLiveSessions`: iterates through the sessions, calculates durations, interaction counts, last active times, populates `LiveStudentInfo` for each, and generates `StudentAlert`s based on the threshold criteria (idle, stuck, struggling). It will return a map of steps to students and a list of alerts.

2.  **Refactor Main Function**: Use `run_in_bash_session` to replace the body of `GetActivityLiveDetail` in `internal/delivery/http/handler/dashboard.go` with calls to these four new helper functions, significantly reducing its length and complexity while preserving the original request handling and response formatting logic.

3.  **Verify Syntax**: Use `run_in_bash_session` to run `go build ./internal/delivery/http/handler/...` to verify that the refactored code compiles and that there are no syntax or type errors. Also use `read_file` or `cat` to inspect the updated file content.

4.  **Run Tests**: Use `run_in_bash_session` to execute the backend tests with `go test $(go list ./... | grep -v /scripts)` to ensure no regressions were introduced.

5.  **Pre-commit steps**: Complete pre-commit steps to ensure proper testing, verification, review, and reflection are done.

6.  **Submit**: Commit and push the changes.
