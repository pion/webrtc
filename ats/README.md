# Acceptance Test Specifications

TL:DR; From the project's root `./ats/run`

This directory contains Acceptance Test Specifications AKA black box tests.
The tests are running over a lab managed by docker-compose and using playwright
for browser automation.

The `ats` directory includes a special directory `infra` with the infrastructure
required by the tests.
For example, there's a `./infra/pion` directory with a Dockerfile and an SSH
config file.

Unlike production containers, lab containers' entry point
often includes setup code usually found in the Dockerfile.
This is done for flexibility and speed as we want the lab to use the latest source

The script support some old style options, use `./ats/run -h` to see the 
all the options. It also accepts one of more argument with a directory name.

## The setup

We use [playwright](https://playwright.dev) as the test runner.
In addition to browser automation the runner uses SSH to control the services.
Thanks to compose, a name of a service is also its address so 

The runner supports one environment variable - `PWOPTS` - one can use to pass
options to playwright. The default is `-x` stopping the test on the first
failure. It's rigged this way becuase ATSs are usually complex scenarios.
Unlike unit tests, where each test function is independent, here each functions
is a step in one test specification. Once a step failed, `-x` makes playwright ignore
the rest of the file.

To get help on playwright options run:

```bash
docker compose -f ats/data-channels/lab.yaml --project-directory . run --entrypoint "npx playwright test --help" runner
```

The above command uses the lab from the data-channels tests to bring up a runner
and override its entrypoint. Instead of running the specs, get help on playwright.
We use `--project-directory .` in all the tests to ensure the relative paths are
relative to the project's root. 

## Adding a test

To create the `fubar` test, create a new directory in `./ats/fubar` and start working
one your `lab.yaml`. If your tests include a browser client your runner is
best using the image from `infra/playwright`:

```yaml
version: "3.9"
services:
  runner:
    build: 
      context: .
      dockerfile: ./ats/infra/playwright/Dockerfile
    volumes:
      - ./ats/fubar:/specs
      - ./ats/result:/result
    environment:
      PWOPTS: ${PWOPTS--x}
```

Not that we are mapping `/specs` to `ats/fubar` as the first is where the image looks
for specs. Once you've added your specs file to `./ats/fubar` it will run whener you 
`./ats/run`. In fact, all a sub-directory of ats needs is a `lab.yaml` file and the
run script will try to bring it up.

If you want to run just fubar you can use `./ats/run ats/fubar`.
To run a clean build of fubar, ignoring all cache, use `-z`.
