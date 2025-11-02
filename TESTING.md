# TESTING

Run:
```
make test
go test ./... -cover
```
Unit tests cover parser, classifier, and crawler (with httptest). Integration tests can be layered over the server using `httptest` in a follow-up.


## Integration tests
These hit real sites and are **optional**:
```
go test ./... -tags=integration -v
```
They are skipped if the target rejects bots or the network is unavailable.
