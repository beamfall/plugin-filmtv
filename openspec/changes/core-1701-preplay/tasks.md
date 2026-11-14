## 1. Policy

- [x] 1.1 Add `PlanPrePlay`, `PrePlayReader`, `PrePlayRequest`, `PrePlayPlan` and `PrePlayExtra`

## 2. Tests

- [x] 2.1 Unit-test eligibility, trailer detection, exclusion, ordering, the cap and reader errors

## 3. Verify and close

- [x] 3.1 `openspec validate --all --strict`
- [ ] 3.2 `script/gate.sh`
- [ ] 3.3 `openspec archive core-1701-preplay` after landing
