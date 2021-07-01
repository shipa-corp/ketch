#!/usr/bin/env bats

# To run locally:
# export KETCH_EXECUTABLE_PATH=<location of ketch binary>
# assure you have a kubernetes cluster running w/ traefik, cert manager, etc. (see ketch getting started docs)
# assure the ketch cli is compiled (make ketch)
# assure you have bats installed locally (via apt, brew, etc.)
# ./cli_tests/app.sh

setup() {
    if [[ -z "${KETCH_EXECUTABLE_PATH}" ]]; then
    KETCH=$(pwd)/bin/ketch
  else
    KETCH="${KETCH_EXECUTABLE_PATH}"
  fi
  INGRESS=$(kubectl get svc traefik -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
  FRAMEWORK="myframework"
  APP_IMAGE="gcr.io/shipa-ci/sample-go-app:latest"
  APP_NAME="sample-app"
  CNAME="my-cname.com"
  TEST_ENVVAR_KEY="FOO"
  TEST_ENVVAR_VALUE="BAR"
}

@test "help" {
  result="$($KETCH help)"
  echo "RECEIVED:" $result
  [[ $result =~ "For details see https://theketch.io" ]]
  [[ $result =~ "Available Commands" ]]
  [[ $result =~ "Flags" ]]
}

@test "framework add" {
  result=$($KETCH framework add "$FRAMEWORK" --ingress-service-endpoint "$INGRESS" --ingress-type "traefik")
  echo "RECEIVED:" $result
  [[ $result =~ "Successfully added!" ]]
}

@test "framework list" {
  result=$($KETCH framework list)
  headerRegex="NAME[ \t]+STATUS[ \t]+NAMESPACE[ \t]+INGRESS TYPE[ \t]+INGRESS CLASS NAME[ \t]+CLUSTER ISSUER[ \t]+APPS"
  dataRegex="$FRAMEWORK[ \t]+ketch-$FRAMEWORK[ \t]+traefik[ \t]+traefik"
  echo "RECEIVED:" $result
  [[ $result =~ $headerRegex ]]
  [[ $result =~ $dataRegex ]]
}

@test "framework update" {
  result=$($KETCH framework update "$FRAMEWORK" --app-quota-limit 1)
  echo "RECEIVED:" $result
  [[ $result =~ "Successfully updated!" ]]
}

@test "framework export" {
  run $KETCH framework export "$FRAMEWORK"
  result=$(cat framework.yaml)
  echo "RECEIVED:" $result
  [[ $result =~ "name: $FRAMEWORK" ]]
  [[ $result =~ "namespace: ketch-$FRAMEWORK" ]]
  [[ $result =~ "appQuotaLimit: 1" ]]
  rm -f framework.yaml
}

@test "app deploy" {
  run $KETCH app deploy "$APP_NAME" --framework "$FRAMEWORK" -i "$APP_IMAGE"
  [[ $status -eq 0 ]]
}

@test "app list" {
  result=$($KETCH app list)
  headerRegex="NAME[ \t]+FRAMEWORK[ \t]+STATE[ \t]+ADDRESSES[ \t]+BUILDER[ \t]+DESCRIPTION"
  dataRegex="$APP_NAME[ \t]+$FRAMEWORK[ \t]+(created|running)"
  echo "RECEIVED:" $result
  [[ $result =~ $headerRegex ]]
  [[ $result =~ $dataRegex ]]
}

@test "app info" {
  result=$($KETCH app info "$APP_NAME")
  headerRegex="DEPLOYMENT VERSION[ \t]+IMAGE[ \t]+PROCESS NAME[ \t]+WEIGHT[ \t]+STATE[ \t]+CMD"
  dataRegex="1[ \t]+$APP_IMAGE[ \t]+web[ \t]+100%[ \t]+created[ \t]"
  echo "RECEIVED:" $result
  [[ $result =~ $headerRegex ]]
  [[ $result =~ $dataRegex ]]
}

@test "app stop" {
  result=$($KETCH app stop "$APP_NAME")
  echo "RECEIVED:" $result
  [[ $result =~ "Successfully stopped!" ]]
}

@test "app start" {
  result=$($KETCH app start "$APP_NAME")
  echo "RECEIVED:" $result
  [[ $result =~ "Successfully started!" ]]
}

@test "app log" {
  run $KETCH app log "$APP_NAME"
  [[ $status -eq 0 ]]
}

@test "builder list" {
  result=$($KETCH builder list)
  headerRegex="VENDOR[ \t]+IMAGE[ \t]+DESCRIPTION"
  dataRegex="Google[ \t]+gcr.io/buildpacks/builder:v1[ \t]+GCP Builder for all runtimes"
  echo "RECEIVED:" $result
  [[ $result =~ $headerRegex ]]
  [[ $result =~ $dataRegex ]]
}

@test "cname add" {
  run $KETCH cname add "$CNAME" --app "$APP_NAME"
  [[ $status -eq 0 ]]
  result=$($KETCH app info "$APP_NAME")
  echo "RECEIVED:" $result
  [[ $result =~ "Address: http://$CNAME" ]]
}

@test "cname remove" {
  run $KETCH cname remove "$CNAME" --app "$APP_NAME"
  [[ $status -eq 0 ]]
  result=$($KETCH app info "$APP_NAME")
  echo "RECEIVED:" $result
  [[ ! $result =~ "Address: http://$CNAME" ]]
}

@test "unit add" {
 run $KETCH unit add 1 --app "$APP_NAME"
 [[ $status -eq 0 ]]
 result=$(kubectl describe apps $APP_NAME)
 echo "RECEIVED:" $result
 [[ $result =~ "Units:  2" ]] # note two spaces
}

@test "unit remove" {
 run $KETCH unit remove 1 --app "$APP_NAME"
 [[ $status -eq 0 ]]
  result=$(kubectl describe apps $APP_NAME)
  echo "RECEIVED:" $result
 [[ $result =~ "Units:  1" ]] # note two spaces
}

@test "unit set" {
 run $KETCH unit set 3 --app "$APP_NAME"
 [[ $status -eq 0 ]]
  result=$(kubectl describe apps $APP_NAME)
  echo "RECEIVED:" $result
 [[ $result =~ "Units:  3" ]] # note two spaces
}

@test "env set" {
  run $KETCH env set "$TEST_ENVVAR_KEY=$TEST_ENVVAR_VALUE" --app "$APP_NAME"
  [[ $status -eq 0 ]]
}

@test "env get" {
  result=$($KETCH env get "$TEST_ENVVAR_KEY" --app "$APP_NAME")
  echo "RECEIVED:" $result
  [[ $result =~ "$TEST_ENVVAR_VALUE" ]]
}

@test "env unset" {
  run $KETCH env unset "$TEST_ENVVAR_KEY" --app "$APP_NAME"
  [[ $status -eq 0 ]]
  result=$($KETCH env get "$TEST_ENVVAR_KEY" --app "$APP_NAME")
  echo "RECEIVED:" $result
  [[ ! $result =~ "$TEST_ENVVAR_VALUE" ]]
}

@test "app remove" {
  result=$($KETCH app remove "$APP_NAME")
  echo "RECEIVED:" $result
  [[ $result =~ "Successfully removed!" ]]
}

@test "framework remove" {
  result=$(echo "ketch-$FRAMEWORK" | $KETCH framework remove "$FRAMEWORK")
  echo "RECEIVED:" $result
  [[ $result =~ "Framework successfully removed!" ]]
}
