# Acceptance Test Procedure

TL:DR; From the project's root `./acceptance_tests/run`

This directory contains Acceptance Test Procedure AKA black box tests.
The tests are running over a lab managed by docker-compose and using playwright
for browser automation.

The `acceptance_tests` directory includes a special directory `infra` with the infrastructure
required by the tests.
For example, there's a `./infra/pion` directory with a Dockerfile and an SSH
config file.

Unlike production containers, lab containers' entry point
often includes setup code usually found in the Dockerfile.
This is done for flexibility and speed as we want the lab to use the latest source

The script support some old style options, use `./acceptance_tests/run -h` to see the 
all the options. It also accepts one of more argument with a directory name.

## The setup

We use [playwright](https://playwright.dev) as the test runner.
In addition to browser automation the runner uses SSH to control the services.
Thanks to compose, a name of a service is also its address so 

The runner supports one environment variable - `PWOPTS` - one can use to pass
options to playwright. The default is `-x` stopping the test on the first
failure. It's rigged this way becuase ATPs are usually complex scenarios.
Unlike unit tests, where each test function is independent, here each functions
is a step in one test procedure. Once a step failed, `-x` makes playwright ignore
the rest of the file.

To get help on playwright options run:

```bash
docker compose -f acceptance_tests/data-channels/lab.yaml --project-directory . run --entrypoint "npx playwright test --help" runner
```

The above command uses the lab from the data-channels tests to bring up a runner
and override its entrypoint. Instead of running the specs, get help on playwright.
We use `--project-directory .` in all the tests to ensure the relative paths are
relative to the project's root. 

## Adding a test

To create the `fubar` test, create a new directory in `./acceptance_tests/fubar` and start working
one your `lab.yaml`. If your tests include a browser client your runner is
best using the image from `infra/playwright`:

```yaml
version: "3.9"
services:
  runner:
    build: 
      context: .
      dockerfile: ./acceptance_tests/infra/playwright/Dockerfile
    volumes:
      - ./acceptance_tests/fubar:/specs
      - ./acceptance_tests/result:/result
    environment:
      PWOPTS: ${PWOPTS--x}
```

Not that we are mapping `/specs` to `acceptance_tests/fubar` as the first is where the image looks
for specs. Once you've added your specs file to `./acceptance_tests/fubar` it will run whener you 
`./acceptance_tests/run`. In fact, all a sub-directory of acceptance_tests needs is a `lab.yaml` file and the
run script will try to bring it up.

If you want to run just fubar you can use `./acceptance_tests/run acceptance_tests/fubar`.
To run a clean build of fubar, ignoring all cache, use `-z`.


