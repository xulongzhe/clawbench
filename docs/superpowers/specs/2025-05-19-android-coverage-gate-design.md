# Android Coverage Gate Design

**Date:** 2025-05-19
**Issue:** #28
**Branch:** fix/android-coverage-gate

## Summary

Add two-tier coverage gate for Android, mirroring the existing Go and Frontend gates (PR #19).

- **Tier 1 (Project Gate):** per-class coverage >= adjusted baseline% - 1.5%
- **Tier 2 (Diff Coverage):** changed lines coverage >= 80% (strict)

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Tier 2 threshold | Strict 80% | Same as Go/Frontend, forces test coverage on new code |
| JaCoCo config | AGP built-in (`testCoverageEnabled true`) | AGP 8.2 supports it natively, no manual plugin needed |
| Coverage scope | `app` module only | Single-module project, no libraries |
| Script language | Bash + embedded Python | Consistent with Go/Frontend scripts |
| Coverage format | JaCoCo XML | Standard, well-structured, supports class-level and line-level data |

## Architecture

### 1. JaCoCo Configuration (build.gradle)

Add to `android/app/build.gradle`:

```groovy
android {
    buildTypes {
        debug {
            testCoverageEnabled true
        }
    }
}

tasks.register('jacocoTestReport', JacocoReport) {
    dependsOn 'testDebugUnitTest'
    reports {
        xml.required = true
        html.required = true
    }
    def excludes = [
        '**/R.class', '**/R$*.class', '**/BuildConfig.*',
        '**/Manifest*.*'
    ]
    classDirectories.setFrom(files([
        fileTree(dir: "$buildDir/intermediates/javac/debug/classes",
                 excludes: excludes)
    ]))
    sourceDirectories.setFrom(files(['app/src/main/java']))
    executionData.setFrom(files(["$buildDir/jacoco/testDebugUnitTest.exec"]))
}
```

Command to run: `cd android && JAVA_HOME=... ./gradlew jacocoTestReport`

Output: `android/app/build/reports/jacoco/jacocoTestReport/jacocoTestReport.xml`

### 2. JaCoCo XML Format

Key structures used by the gate script:

**Tier 1 — Class-level counters:**
```xml
<class name="com/clawbench/app/PortForwardService" sourcefilename="PortForwardService.java">
  <counter type="LINE" missed="45" covered="120"/>
</class>
```

**Tier 2 — Line-level data:**
```xml
<sourcefile name="PortForwardService.java">
  <line nr="42" mi="0" ci="3"/>   <!-- mi=missed, ci=covered instructions -->
  <line nr="43" mi="2" ci="0"/>
</sourcefile>
```

### 3. Gate Script: `scripts/check-android-coverage.sh`

Structure mirrors `check-go-coverage.sh` and `check-frontend-coverage.sh`:

```
Bash wrapper:
  1. Run tests with coverage (or --skip-test)
  2. Detect merge-base
  3. Download baseline artifact
  4. Call embedded Python script

Python script:
  Tier 1: Parse JaCoCo XML → per-class LINE counters → compare with adjusted baseline
  Tier 2: Parse JaCoCo XML → sourcefile/line level data × git diff → diff coverage %
```

**Tier 1 calculation:**
- Parse all `<class>` elements from JaCoCo XML
- For each class: `covered = counter[@type="LINE"]/@covered`, `missed = counter[@type="LINE"]/@missed`
- Per-class coverage = `covered / (covered + missed) * 100`
- Adjusted baseline: exclude entries overlapping with git-deleted lines (same as Go script)
- Gate: `current% >= baseline% - 1.5%`

**Tier 2 calculation:**
- Parse all `<sourcefile>/<line>` elements from JaCoCo XML
- Build `line_coverage` dict: `file -> {line_nr: covered_bool}` where `covered = ci > 0`
- Cross-reference with git diff added lines (`.java` files, exclude `*Test.java`)
- Gate: diff coverage >= 80%

**Path normalization:**
- JaCoCo XML `<sourcefile name="PortForwardService.java">` is within `<package name="com/clawbench/app">`
- Git diff produces `android/app/src/main/java/com/clawbench/app/PortForwardService.java`
- Mapping: `package/@name + "/" + sourcefile/@name` → `com/clawbench/app/PortForwardService.java`
- Git diff file → extract relative path after `com/clawbench/app/` for matching

### 4. CI Workflow: `coverage-android` job

```yaml
coverage-android:
  name: Coverage Gate (Android)
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
      with:
        fetch-depth: 0

    - name: Download main baseline
      if: github.event_name == 'pull_request'
      uses: dawidd6/action-download-artifact@v8
      with:
        workflow: ci.yml
        branch: main
        name: main-android-coverage
        path: .clawbench/baseline/
      continue-on-error: true

    - uses: actions/setup-java@v4
      with:
        distribution: 'temurin'
        java-version: '17'

    - uses: android-actions/setup-android@v3

    - name: Install Android SDK components
      run: sdkmanager "platforms;android-34" "build-tools;34.0.0"

    - name: Run coverage gate check
      run: ./scripts/check-android-coverage.sh

    - name: Upload coverage data
      if: github.event_name == 'push' && github.ref == 'refs/heads/main'
      uses: actions/upload-artifact@v4
      with:
        name: main-android-coverage
        path: android/app/build/reports/jacoco/jacocoTestReport/jacocoTestReport.xml
        retention-days: 90
```

### 5. Baseline Artifact Flow

Same as Go/Frontend:

1. **Main push** → runs tests → uploads `jacocoTestReport.xml` as `main-android-coverage` artifact
2. **PR** → downloads `main-android-coverage` into `.clawbench/baseline/` → Tier 1 compares against it
3. **Local** → falls back to `.clawbench/baseline/` → `gh run download` → skip Tier 1

### 6. Files Changed

| File | Change |
|------|--------|
| `android/app/build.gradle` | Add `testCoverageEnabled true` + `jacocoTestReport` task |
| `scripts/check-android-coverage.sh` | New file, two-tier gate script |
| `.github/workflows/ci.yml` | Add `coverage-android` job |
| `AGENTS.md` | Update coverage gate docs to include Android |

## Edge Cases

- **No JaCoCo XML found** → error with instructions to run tests first
- **No merge-base** → skip Tier 2
- **No baseline artifact** → skip Tier 1
- **Class with 0% coverage in JaCoCo** → JaCoCo omits it entirely; Tier 2 still catches uncovered changed lines
- **Auto-generated classes (R, BuildConfig)** → excluded from JaCoCo report via `excludes` filter
- **Test files changed** → excluded from Tier 2 (`*Test.java`, `*Test.kt`)
