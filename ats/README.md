# Acceptance Test Specifications

TL:DR; From the project's root `./ats/test`

This folder contains automated acceptance test specifications. 
The tests are using docker-compose for lab setup and playwright
for end-to-end and browser testing.

The script support some old style options, use `./ats/test -h` to see the 
all the options. It also accepts one of more argument with a folder name.

## The runner

We use [playwright](https://playwright.dev) as the test runner and use
its syntax and expectations. To pass options to playwright use the 
`PWARGS` enviornment variable. I use it to get the tests to stop
after the first failure and keep the logs short:

```
PWARGS=-x ./ats/test ./ats/accept
```

Run `npx playwright test --help` for its list of options
