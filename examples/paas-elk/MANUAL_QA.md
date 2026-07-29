# Manual QA: PaaS ELK

Этот сценарий предназначен для ручной проверки локально собранного провайдера.
Существующий репозиторный E2E-набор не считается источником результата.

## Границы проверки

Проверяются только:

- блок `elk` ресурса и data source `aws_paas_service`;
- CRUD, import и drift ресурса `aws_paas_logstash_pipeline`;
- отсутствие регрессии в существующих PaaS schema/unit tests.

Snapshot repositories, backup resources и изменения других PaaS-сервисов в этот
прогон не входят.

## Предусловия

1. Используется отдельный QA-проект K2 Cloud.
2. Подсеть имеет маршрут в Интернет: без него развёртывание ELK не завершится.
3. QA runner имеет маршрут/VPN/bastion до private IP ELK; security group
   разрешает необходимый трафик к Elasticsearch, Kibana и тестовому Logstash
   HTTP input на портах `4567` и `5678` только из доверенной сети.
4. Установлены `terraform`, `jq`, `curl` и `c2-paas`.
5. Загружен полный K2 Cloud `c2rc`: одного `TF_VAR_region` недостаточно,
   `c2-paas` получает PaaS endpoint из `PAAS_URL`.
6. Проверка выполняется в отдельном каталоге с правами `0700`; state и его
   backup содержат sensitive-значения и не должны попадать в Git или QA-отчёт.
7. Следующие переменные окружения содержат реальные значения выделенного стенда:

```bash
set -euo pipefail
umask 077

: "${AWS_ACCESS_KEY_ID:?AWS_ACCESS_KEY_ID is required}"
: "${AWS_SECRET_ACCESS_KEY:?AWS_SECRET_ACCESS_KEY is required}"
: "${EC2_URL:?EC2_URL is required}"
: "${PAAS_URL:?PAAS_URL is required}"
: "${K2_QA_ELK_PASSWORD:?K2_QA_ELK_PASSWORD is required}"
: "${K2_QA_SSH_KEY_NAME:?K2_QA_SSH_KEY_NAME is required}"
: "${K2_QA_SUBNET_IDS_JSON:?K2_QA_SUBNET_IDS_JSON is required}"
: "${K2_QA_SECURITY_GROUP_IDS_JSON:?K2_QA_SECURITY_GROUP_IDS_JSON is required}"

repo_root="$(git rev-parse --show-toplevel)"
qa_work_dir="$(mktemp -d "${TMPDIR:-/tmp}/paas-elk-qa.XXXXXX")"
provider_dir="$qa_work_dir/providers"
mkdir -p "$provider_dir"
chmod 700 "$qa_work_dir" "$provider_dir"

(
  cd "$repo_root"
  go build \
    -o "$provider_dir/terraform-provider-rockitcloud_v0.0.0" \
    .
)
cp "$repo_root"/examples/paas-elk/*.tf "$qa_work_dir"/

cat >"$qa_work_dir/terraformrc" <<EOF
provider_installation {
  dev_overrides {
    "c2devel/rockitcloud" = "$provider_dir"
  }

  direct {}
}
EOF

export TF_CLI_CONFIG_FILE="$qa_work_dir/terraformrc"
export TF_VAR_region=ru-msk
export TF_VAR_elk_password="$K2_QA_ELK_PASSWORD"
export TF_VAR_ssh_key_name="$K2_QA_SSH_KEY_NAME"
export TF_VAR_subnet_ids="$K2_QA_SUBNET_IDS_JSON"
export TF_VAR_security_group_ids="$K2_QA_SECURITY_GROUP_IDS_JSON"
export TF_VAR_elk_options='{"node.attr.qa":"terraform"}'
run_suffix="$(date +%m%d%H%M%S)"
export TF_VAR_service_name="elkqa-$run_suffix"
export TF_VAR_pipeline_name="terraform-qa-$run_suffix"
pipeline_name_initial="$TF_VAR_pipeline_name"

cd "$qa_work_dir"
trap 'rm -f terraform.tfplan; unset TF_VAR_elk_password K2_QA_ELK_PASSWORD' EXIT
```

`K2_QA_SUBNET_IDS_JSON` и `K2_QA_SECURITY_GROUP_IDS_JSON` должны быть JSON-массивами
строк. Команды выполняются в созданном `qa_work_dir`, а не внутри Git worktree.
Секреты не сохраняются в `terraform.tfvars` и не прикладываются к отчёту.
Fixture по умолчанию использует `m5.large`: live API отклонил `c5.large` из-за
недостатка памяти (4 GiB при минимуме ELK 8 GiB). Ключ `node.attr.qa` в
`TF_VAR_elk_options` принят live API и используется для проверки create-only
семантики; придумывать другие version-specific keys не требуется.
Нельзя сохранять или прикладывать сырой вывод `terraform show -json`:
sensitive-значения в JSON не маскируются. Каталог нельзя удалять, пока независимая
API-проверка не подтвердила удаление облачных ресурсов.

## P0: статическая проверка

`dev_overrides` уже указывает на собранный бинарник; повторный `terraform init`
не требуется и не должен скачивать опубликованный provider:

```bash
terraform fmt -check
terraform validate
```

Ожидается успешная валидация без обращения к PaaS Create API. Warning о
`Provider development overrides are in effect` ожидаем.

## P0: read-only preflight

До создания ресурсов отдельно проверить endpoint, credentials и PaaS read
permission:

```bash
c2-paas ListServices serviceType elk |
  jq -e '.services | type == "array"'
```

Команда обязана вернуть массив, включая пустой. `AccessDenied`,
`InvalidAccessKey`, DNS/TLS error или обращение не к значению `PAAS_URL` являются
ошибкой стенда, а не успешным отрицательным ELK-тестом.

## P0: создание ELK и pipeline

```bash
terraform plan -out=terraform.tfplan
terraform apply terraform.tfplan
rm -f terraform.tfplan

terraform output -raw elk_id
terraform output -raw elk_status
terraform output -raw elk_service_type
terraform output -raw elk_service_class
terraform output -raw elk_version_from_data_source
terraform output -raw pipeline_id

[ -n "$(terraform output -raw elk_id)" ]
[ -n "$(terraform output -raw pipeline_id)" ]
```

Ожидается:

- непустые service ID и pipeline ID;
- `elk_status = READY`;
- `elk_service_type = elk`;
- `elk_service_class = logging`;
- версия из data source равна `8.17`;
- одна ELK instance имеет статус `READY`;
- data volume имеет размер 32 GiB;
- endpoints сервиса непустые;
- пароль не появляется в обычном выводе plan/apply.

Машинная проверка state:

```bash
terraform show -json | jq -e '
  .values.root_module.resources[]
  | select(.address == "aws_paas_service.elk")
  | .values
  | .service_type == "elk"
    and .service_class == "logging"
    and .status == "READY"
    and .elk[0].version == "8.17"
    and .elk[0].options["node.attr.qa"] == "terraform"
    and .data_volume[0].size == 32
    and ([.instances[] | select(.status == "READY")] | length) == 1
    and (.endpoints | length) > 0
'

terraform show -json | jq -e \
  --arg pipeline_name "$TF_VAR_pipeline_name" '
  .values.root_module.resources[]
  | select(.address == "aws_paas_logstash_pipeline.qa")
  | .values
  | (.pipeline_id | length) > 0
    and .name == $pipeline_name
'
```

DescribeService и data source могут не вернуть `password` и `options`.
Проверка `options` выше относится к существующему resource state: provider
сохраняет create input при таком API-ответе.

## P0: идемпотентность

```bash
terraform plan -detailed-exitcode
```

Ожидаемый exit code — `0`, итог: `0 to add, 0 to change, 0 to destroy`.

## P0: in-place update pipeline

```bash
pipeline_id_before="$(terraform output -raw pipeline_id)"
export TF_VAR_pipeline_configuration='input { http { port => 5678 tags => ["terraform", "qa", "second"] } }'

terraform plan -out=terraform.tfplan
terraform show -json terraform.tfplan | jq -e '
  .resource_changes[]
  | select(.address == "aws_paas_logstash_pipeline.qa")
  | .change.actions == ["update"]
'
terraform apply terraform.tfplan
rm -f terraform.tfplan

pipeline_id_after="$(terraform output -raw pipeline_id)"
test "$pipeline_id_before" = "$pipeline_id_after"
test "$(terraform output -raw elk_status)" = "READY"
terraform plan -detailed-exitcode
```

Ожидается update без replacement, неизменный pipeline ID и пустой повторный plan.

## P1: работоспособность HTTP input

После in-place update pipeline слушает порт `5678`. С машины в доверенной сети
отправить уникальное событие на первую не-arbitrator instance:

```bash
logstash_host="$(
  terraform output -json elk_instances |
    jq -er '[.[] | select(.role != "arbitrator")][0].private_ip // empty'
)" || exit 1
event_id="terraform-qa-$(date +%s)"

[ -n "$logstash_host" ] || exit 1
curl --fail --silent --show-error \
  --request POST \
  --header 'Content-Type: application/json' \
  --data "{\"event_id\":\"$event_id\",\"source\":\"terraform-provider-qa\"}" \
  "http://$logstash_host:5678"
```

HTTP-запрос должен завершиться успешно. В Kibana Stack Monitoring для pipeline
с именем из `TF_VAR_pipeline_name` счётчик Events In должен увеличиться; в
QA-отчёт записывается
только `event_id`, без содержимого state и pipeline configuration.

## P0: replacement при rename

```bash
export TF_VAR_pipeline_name="${pipeline_name_initial}-renamed"
terraform plan -out=terraform.tfplan
terraform show -json terraform.tfplan | jq -e '
  .resource_changes[]
  | select(.address == "aws_paas_logstash_pipeline.qa")
  | .change.actions == ["delete", "create"]
'
rm -f terraform.tfplan
export TF_VAR_pipeline_name="$pipeline_name_initial"
```

Apply не нужен. Имя отсутствует в Modify API, поэтому plan обязан показать
replacement.

## P0: import pipeline

Import проверяется через временный state-адрес. Исходная привязка остаётся в
state и автоматически восстанавливается при ошибке или прерывании:

```bash
elk_id="$(terraform output -raw elk_id)"
pipeline_id="$(terraform output -raw pipeline_id)"

restore_state_binding() {
  original_address="$1"
  canonical_address="$2"

  if terraform state list | grep -Fxq "$original_address"; then
    terraform state rm "$canonical_address" >/dev/null 2>&1 || true
    terraform state mv "$original_address" "$canonical_address"
  fi
}

restore_import_bindings() {
  restore_state_binding \
    aws_paas_logstash_pipeline.qa_original \
    aws_paas_logstash_pipeline.qa
  restore_state_binding \
    aws_paas_service.elk_original \
    aws_paas_service.elk
}

cleanup_qa_session() {
  restore_import_bindings
  rm -f terraform.tfplan
  unset TF_VAR_elk_password K2_QA_ELK_PASSWORD
}

trap cleanup_qa_session EXIT
trap 'exit 130' INT TERM

terraform state mv \
  aws_paas_logstash_pipeline.qa \
  aws_paas_logstash_pipeline.qa_original
terraform import aws_paas_logstash_pipeline.qa "$elk_id/$pipeline_id"
terraform plan \
  -target=aws_paas_logstash_pipeline.qa \
  -detailed-exitcode
terraform state rm aws_paas_logstash_pipeline.qa
terraform state mv \
  aws_paas_logstash_pipeline.qa_original \
  aws_paas_logstash_pipeline.qa
```

Ожидается восстановление `service_id`, `pipeline_id`, `name` и `configuration`;
targeted plan имеет exit code `0`, после чего исходная привязка возвращена.

Отдельно проверить ошибки формата:

```bash
terraform state mv \
  aws_paas_logstash_pipeline.qa \
  aws_paas_logstash_pipeline.qa_original

for invalid_id in \
  "$elk_id" \
  "/$pipeline_id" \
  "$elk_id/" \
  "$elk_id/$pipeline_id/extra"
do
  set +e
  import_output="$(
    terraform import aws_paas_logstash_pipeline.qa "$invalid_id" 2>&1
  )"
  import_status=$?
  set -e

  if [ "$import_status" -eq 0 ]; then
    terraform state rm aws_paas_logstash_pipeline.qa
    exit 1
  fi

  grep -F 'service_id/pipeline_id' <<<"$import_output"
  if terraform state list |
    grep -Fxq aws_paas_logstash_pipeline.qa
  then
    exit 1
  fi
done

terraform state mv \
  aws_paas_logstash_pipeline.qa_original \
  aws_paas_logstash_pipeline.qa
```

Каждая команда должна завершиться ошибкой с форматом
`service_id/pipeline_id`; исходная привязка всё время остаётся под временным
адресом и после цикла возвращается.

## P0: import ELK service

Проверить унаследованный generic import без изменения облачного сервиса и без
потери исходного state:

```bash
elk_id="$(terraform output -raw elk_id)"

terraform state mv \
  aws_paas_service.elk \
  aws_paas_service.elk_original
terraform import aws_paas_service.elk "$elk_id"

terraform show -json | jq -e \
  --arg elk_id "$elk_id" '
  .values.root_module.resources[]
  | select(.address == "aws_paas_service.elk")
  | .values
  | .id == $elk_id
    and .service_type == "elk"
    and .service_class == "logging"
    and .elk[0].version == "8.17"
'

set +e
terraform plan \
  -target=aws_paas_service.elk \
  -out=terraform.tfplan \
  -detailed-exitcode
import_plan_status=$?
set -e
test "$import_plan_status" -eq 2

terraform show -json terraform.tfplan | jq -e '
  .resource_changes[]
  | select(.address == "aws_paas_service.elk")
  | .change
  | .actions == ["delete", "create"]
    and any(.replace_paths[]?; .[-1] == "password")
    and any(.replace_paths[]?; .[-1] == "options")
'
rm -f terraform.tfplan

terraform state rm aws_paas_service.elk
terraform state mv \
  aws_paas_service.elk_original \
  aws_paas_service.elk
terraform plan -detailed-exitcode
```

Import должен выбрать manager по полученному из API `serviceType = elk`,
восстановить ELK block и API-readable поля. В этом сценарии заданы `password` и
`options`, но live API не возвращает эти create inputs, поэтому targeted plan
обязан показать replacement по обоим полям; применять его нельзя. Кроме того,
`delete_interfaces_on_destroy` является локальным delete-only флагом и не
восстанавливается API. После проверки импортированный state удаляется, исходная
привязка с password, options и delete-флагом возвращается, а полный plan снова имеет
exit code `0`.

Import без API-elided create inputs с `ImportStateVerify` отдельно выполняется
credentialed acceptance-тестом; там игнорируются
`delete_interfaces_on_destroy` и `arbitrator_required`, поскольку ответы
DescribeService их не восстанавливают.

## P0: drift и идемпотентный delete

Прямое удаление асинхронно изменяет родительский сервис. Перед Terraform нужно
дождаться одновременно статуса `READY` и исчезновения pipeline из списка:

```bash
wait_for_pipeline_absent() {
  for attempt in $(seq 1 360); do
    service_json="$(
      c2-paas DescribeService serviceId "$elk_id"
    )" || service_json=""
    service_status="$(
      jq -er '.service.status' <<<"$service_json" 2>/dev/null
    )" || service_status=""
    pipelines_json="$(
      c2-paas ListLogstashPipelines serviceId "$elk_id"
    )" || pipelines_json=""
    pipeline_present="$(
      jq -r --arg pipeline_id "$pipeline_id" \
        'any(.logstashPipelines[]?; .id == $pipeline_id)' \
        <<<"$pipelines_json" 2>/dev/null
    )" || pipeline_present=""

    if [ "$service_status" = "READY" ] &&
      [ "$pipeline_present" = "false" ]; then
      return 0
    fi
    sleep 10
  done

  printf 'ELK did not reach READY with the pipeline absent within 60 minutes\n' >&2
  return 1
}
```

Сначала проверить drift:

```bash
elk_id="$(terraform output -raw elk_id)"
pipeline_id="$(terraform output -raw pipeline_id)"
c2-paas DeleteLogstashPipeline serviceId "$elk_id" pipelineId "$pipeline_id"
wait_for_pipeline_absent || exit 1

terraform plan -out=terraform.tfplan
terraform show -json terraform.tfplan | jq -e '
  .resource_changes[]
  | select(.address == "aws_paas_logstash_pipeline.qa")
  | .change.actions == ["create"]
'
terraform apply terraform.tfplan
rm -f terraform.tfplan
terraform plan -detailed-exitcode
```

Затем проверить 404 непосредственно в Delete:

```bash
elk_id="$(terraform output -raw elk_id)"
pipeline_id="$(terraform output -raw pipeline_id)"
c2-paas DeleteLogstashPipeline serviceId "$elk_id" pipelineId "$pipeline_id"
wait_for_pipeline_absent || exit 1

terraform destroy -refresh=false -target=aws_paas_logstash_pipeline.qa -auto-approve
terraform apply -auto-approve
terraform plan -detailed-exitcode
```

Destroy не должен завершаться ошибкой, а последний apply должен восстановить
pipeline.

## P0: негативные проверки schema и API

Каждый случай выполняется отдельно. После ошибки вернуть исходное значение и
проверить, что state не изменился.

```bash
expect_plan_failure() {
  expected_message="$1"
  shift

  set +e
  validation_output="$("$@" 2>&1)"
  validation_status=$?
  set -e

  if [ "$validation_status" -eq 0 ]; then
    printf 'terraform plan unexpectedly succeeded: %s\n' "$*" >&2
    return 1
  fi

  grep -F "$expected_message" <<<"$validation_output"
}
```

| Случай | Команда или изменение | Ожидаемый результат |
|---|---|---|
| Пустая версия | `expect_plan_failure 'to not be an empty string' env TF_VAR_elk_version= terraform plan -refresh=false -input=false` | Ошибка ELK provider schema до API |
| Пароль короче 8 символов | `expect_plan_failure 'range (8 - 128)' env TF_VAR_elk_password=Abcd123 terraform plan -refresh=false -input=false` | Ошибка ELK provider schema до API |
| Запрещённый символ в пароле | `expect_plan_failure 'to not contain any of' env TF_VAR_elk_password='Abcd!234' terraform plan -refresh=false -input=false` | Ошибка ELK provider schema до API |
| Неизвестная anonymous role | `expect_plan_failure 'to be one of' env TF_VAR_allow_anonymous=true TF_VAR_anonymous_roles='["admin"]' terraform plan -refresh=false -input=false` | Ошибка ELK provider schema до API |
| Более одной anonymous role | `expect_plan_failure 'supports 1 item maximum' env TF_VAR_allow_anonymous=true TF_VAR_anonymous_roles='["viewer","editor"]' terraform plan -refresh=false -input=false` | Ошибка ELK provider schema до API |
| Зарезервированное имя pipeline | `expect_plan_failure 'to not be any of' env TF_VAR_pipeline_name=beats-to-elasticsearch terraform plan -refresh=false -input=false` | Ошибка Logstash provider schema до API |
| Пустая configuration | `expect_plan_failure 'to not be an empty string' env TF_VAR_pipeline_configuration= terraform plan -refresh=false -input=false` | Ошибка Logstash provider schema до API |
| Синтаксически неверная configuration | См. отдельный сценарий ниже | Ошибка PaaS без потери существующего state |

Поддерживаемые версии не зашиты в provider: их проверяет PaaS, чтобы новая версия
не требовала немедленного релиза provider. Разрушительный apply неподдерживаемой
версии над основным стендом не выполняется.

Структурные ограничения — обязательный `data_volume`, запрет `backup_settings`
и `logging`, а также ровно один service block — проверяются credential-free
Go-тестами. Для них QA не изменяет tracked `main.tf`.

Неверную configuration проверить на существующем pipeline:

```bash
pipeline_id_before="$(terraform output -raw pipeline_id)"
set +e
invalid_pipeline_output="$(
  TF_VAR_pipeline_configuration='input {' \
    terraform apply -auto-approve -input=false 2>&1
)"
invalid_pipeline_status=$?
set -e

if [ "$invalid_pipeline_status" -eq 0 ]; then
  exit 1
fi
grep -F 'error modifying PaaS Logstash Pipeline' \
  <<<"$invalid_pipeline_output"
test "$pipeline_id_before" = "$(terraform output -raw pipeline_id)"
terraform plan -detailed-exitcode
```

Apply должен завершиться понятной ошибкой PaaS. ID и сохранённая сервером
configuration остаются прежними; после возврата к валидному значению повторный
plan имеет exit code `0`.

Отдельно зафиксировать известное расхождение live API с опубликованными
многострочными примерами:

```bash
pipeline_id_before="$(terraform output -raw pipeline_id)"
set +e
multiline_pipeline_output="$(
  TF_VAR_pipeline_configuration=$'input {\n  http { port => 5678 }\n}' \
    terraform apply -auto-approve -input=false 2>&1
)"
multiline_pipeline_status=$?
set -e

if [ "$multiline_pipeline_status" -eq 0 ]; then
  exit 1
fi
grep -F 'control characters are not allowed' \
  <<<"$multiline_pipeline_output"
test "$pipeline_id_before" = "$(terraform output -raw pipeline_id)"
terraform plan -detailed-exitcode
```

Provider намеренно принимает любую непустую строку и делегирует синтаксис
Logstash PaaS. Ошибка `control characters are not allowed` для literal newlines
является дефектом облачного API, а не основанием добавлять недокументированный
запрет в Terraform schema. После неуспешного apply pipeline ID и серверная
configuration остаются прежними; рабочий workaround — однострочная строка.

Проверка не-ELK parent изолирована отдельным ресурсом и не меняет основной
pipeline:

```bash
: "${K2_QA_NON_ELK_SERVICE_ID:?K2_QA_NON_ELK_SERVICE_ID is required}"
wrong_service_json="$(
  c2-paas DescribeService serviceId "$K2_QA_NON_ELK_SERVICE_ID"
)"
jq -e '
  .service.status == "READY"
    and .service.serviceType != "elk"
' <<<"$wrong_service_json"

set +e
wrong_parent_output="$(
  TF_VAR_wrong_service_id="$K2_QA_NON_ELK_SERVICE_ID" \
    terraform apply \
    -target='aws_paas_logstash_pipeline.wrong_service[0]' \
    -auto-approve \
    -input=false 2>&1
)"
wrong_parent_status=$?
set -e

if [ "$wrong_parent_status" -eq 0 ]; then
  exit 1
fi
grep -F 'error creating PaaS Logstash Pipeline' \
  <<<"$wrong_parent_output"
if grep -Eiq 'AccessDenied|Unauthorized|InvalidAccessKey' \
  <<<"$wrong_parent_output"
then
  exit 1
fi

if terraform state list |
  grep -Fxq 'aws_paas_logstash_pipeline.wrong_service[0]'
then
  TF_VAR_wrong_service_id="$K2_QA_NON_ELK_SERVICE_ID" \
    terraform destroy \
      -target='aws_paas_logstash_pipeline.wrong_service[0]' \
      -auto-approve
fi

partial_pipeline_ids="$(
  c2-paas ListLogstashPipelines \
    serviceId "$K2_QA_NON_ELK_SERVICE_ID" 2>/dev/null |
    jq -r \
      --arg name "${TF_VAR_pipeline_name}-invalid-target" \
      '.logstashPipelines[]? | select(.name == $name) | .id'
)" || partial_pipeline_ids=""
for partial_pipeline_id in $partial_pipeline_ids; do
  c2-paas DeleteLogstashPipeline \
    serviceId "$K2_QA_NON_ELK_SERVICE_ID" \
    pipelineId "$partial_pipeline_id"
done

terraform plan -detailed-exitcode
```

Ожидается именно ошибка создания pipeline для несовместимого типа сервиса, а не
ошибка credentials/permissions. Любой partial pipeline с уникальным именем
удаляется API-проверкой; временного ресурса в state после cleanup быть не должно.

## P1: anonymous access

Поля анонимного доступа replacement-only, поэтому сценарий выполняется после
основных pipeline-проверок:

```bash
elk_id_before="$(terraform output -raw elk_id)"
export TF_VAR_allow_anonymous=true
export TF_VAR_anonymous_roles='["viewer"]'

terraform plan -out=terraform.tfplan
terraform show -json terraform.tfplan | jq -e '
  .resource_changes[]
  | select(.address == "aws_paas_service.elk")
  | .change.actions == ["delete", "create"]
'
terraform apply terraform.tfplan
rm -f terraform.tfplan

elk_id_after="$(terraform output -raw elk_id)"
test "$elk_id_before" != "$elk_id_after"
c2-paas DescribeService serviceId "$elk_id_after" | jq -e '
  .service.parameters.allowAnonymous == true
    and .service.parameters.anonymousRole == "viewer"
'
terraform plan -detailed-exitcode
```

Публичная документация описывает массив, но live API отклоняет даже одноэлементный
массив и принимает scalar enum. Terraform оставляет list-shaped конфигурацию с
максимум одним элементом и отправляет `viewer` scalar. Повторный plan имеет exit
code `0`. В UI проверить anonymous-доступ к Kibana с выбранной ролью `viewer`.

## P1: options lifecycle

Начальный create уже передал проверенный `node.attr.qa`. Убедиться, что API
omission не стирает значение из существующего resource state, а изменение
create-only map планирует replacement:

```bash
terraform show -json | jq -e '
  .values.root_module.resources[]
  | select(.address == "aws_paas_service.elk")
  | .values.elk[0].options["node.attr.qa"] == "terraform"
'

c2-paas DescribeService serviceId "$(terraform output -raw elk_id)" | jq -e '
  (.service.parameters | has("options") | not)
  or .service.parameters.options["node.attr.qa"] == "terraform"
'

export TF_VAR_elk_options='{"node.attr.qa":"replacement-check"}'
terraform plan -out=terraform.tfplan
terraform show -json terraform.tfplan | jq -e '
  .resource_changes[]
  | select(.address == "aws_paas_service.elk")
  | .change
  | .actions == ["delete", "create"]
    and any(.replace_paths[]?; .[-1] == "options")
'
rm -f terraform.tfplan

export TF_VAR_elk_options='{"node.attr.qa":"terraform"}'
terraform plan -detailed-exitcode
```

Replacement apply не выполняется: create был проверен первоначальным apply, а
ForceNew lifecycle — планом. Data source не обязан восстанавливать поле, которое
DescribeService не вернул.

## P1: HA и arbitrator

Сначала уничтожить single-node стенд. Затем передать три пригодные подсети в
`K2_QA_SUBNET_IDS_JSON` и выполнить:

```bash
terraform destroy -auto-approve
export TF_VAR_subnet_ids="$K2_QA_SUBNET_IDS_JSON"
export TF_VAR_elk_password="$K2_QA_ELK_PASSWORD"
unset TF_VAR_allow_anonymous
unset TF_VAR_anonymous_roles
export TF_VAR_high_availability=true
export TF_VAR_arbitrator_required=true
terraform apply -auto-approve

test "$(terraform output -raw elk_status)" = "READY"
terraform output -json elk_instances | jq -e 'length >= 3'
ha_elk_id="$(terraform output -raw elk_id)"
c2-paas DescribeService serviceId "$ha_elk_id" | jq -e '
  .service.highAvailability == true
    and .service.nodes.main != null
    and .service.nodes.arbitrator != null
    and ([.service.instances[] | select(.role == "arbitrator")] | length) >= 1
    and all(
      .service.instances[]
      | select(.role == "arbitrator")
      | .endpoints[]?;
      (.name | ascii_downcase | test("kibana|logstash") | not)
    )
'
terraform plan -detailed-exitcode
```

API обязан вернуть main и arbitrator topology, не менее одной instance с ролью
`arbitrator` и без прикладных Kibana/Logstash endpoints у неё. В UI повторить
визуальную проверку topology. Повторный plan должен иметь exit code `0`.

## P1: monitoring

Prometheus должен находиться в том же проекте и VPC. Его ID передаётся без
изменения конфигурации provider:

```bash
: "${K2_QA_PROMETHEUS_SERVICE_ID:?K2_QA_PROMETHEUS_SERVICE_ID is required}"
elk_id_before="$(terraform output -raw elk_id)"
export TF_VAR_monitoring_service_id="$K2_QA_PROMETHEUS_SERVICE_ID"

terraform plan -out=terraform.tfplan
terraform show -json terraform.tfplan | jq -e '
  .resource_changes[]
  | select(.address == "aws_paas_service.elk")
  | .change.actions == ["update"]
'
terraform apply terraform.tfplan
rm -f terraform.tfplan

test "$elk_id_before" = "$(terraform output -raw elk_id)"
test "$(terraform output -raw elk_status)" = "READY"
terraform show -json | jq -e \
  --arg monitor_by "$K2_QA_PROMETHEUS_SERVICE_ID" '
  .values.root_module.resources[]
  | select(.address == "data.aws_paas_service.elk")
  | .values.elk[0].monitoring[0]
  | .monitor_by == $monitor_by
    and .monitoring_labels.environment == "qa"
    and .monitoring_labels.managed_by == "terraform"
'
terraform plan -detailed-exitcode
```

Затем изменить `monitoring_labels`, применить конфигурацию и отключить monitoring,
удалив переменную `TF_VAR_monitoring_service_id`:

```bash
export TF_VAR_monitoring_labels='{"environment":"qa","revision":"second"}'
terraform plan -out=terraform.tfplan
terraform show -json terraform.tfplan | jq -e '
  .resource_changes[]
  | select(.address == "aws_paas_service.elk")
  | .change.actions == ["update"]
'
terraform apply terraform.tfplan
rm -f terraform.tfplan

test "$elk_id_before" = "$(terraform output -raw elk_id)"
terraform show -json | jq -e '
  .values.root_module.resources[]
  | select(.address == "data.aws_paas_service.elk")
  | .values.elk[0].monitoring[0].monitoring_labels
  | .environment == "qa" and .revision == "second"
'
terraform plan -detailed-exitcode

unset TF_VAR_monitoring_service_id
terraform plan -out=terraform.tfplan
terraform show -json terraform.tfplan | jq -e '
  .resource_changes[]
  | select(.address == "aws_paas_service.elk")
  | .change.actions == ["update"]
'
terraform apply terraform.tfplan
rm -f terraform.tfplan

test "$elk_id_before" = "$(terraform output -raw elk_id)"
terraform show -json | jq -e '
  .values.root_module.resources[]
  | select(.address == "data.aws_paas_service.elk")
  | (.values.elk[0].monitoring // []) == []
'
terraform plan -detailed-exitcode
```

Все операции должны быть in-place, service ID не меняется, статус возвращается в
`READY`, data source отражает актуальные labels. После каждого apply пустой plan
также подтверждает, что API-elided `password` и `options` сохранились в resource
state, а ELK update не попытался изменить create-only параметры. Unit regression
отдельно проверяет точный whitelist request-полей: `monitoring`, `monitor_by`,
`monitoring_labels`.

## Регрессионный набор без облака

Из каталога `examples/paas-elk`, не меняя текущую директорию:

```bash
(
  set -e
  cd ../..
  go test ./internal/service/paas/services -count=1
  go test ./internal/service/paas -count=1
  go test ./internal/provider -count=1
  go test ./internal/conns -count=1
)
```

Обязательны существующие тесты выбора Elasticsearch и Prometheus manager,
Prometheus import helpers и `Provider().InternalValidate()`. Создание сторонних
PaaS-сервисов только ради регрессии не требуется: это увеличивает стоимость и
выходит за ELK scope.

## Автоматизированный acceptance harness

Постоянный acceptance harness запускается из корня репозитория. Тест полного
lifecycle сам создаёт изолированные VPC, subnet и security group, проверяет ELK,
data source, pipeline CRUD/import/replacement/drift и затем удаляет созданную
инфраструктуру:

```bash
TF_ACC=1 go test ./internal/service/paas \
  -run '^TestAccPaaSELK_logstashPipelineBasic$' \
  -count=1 -v -timeout 45m
```

Тест параметров использует существующие Prometheus, security group и subnet.
Перед запуском должны быть экспортированы:

- `K2_ACC_ELK_MONITOR_SERVICE_ID` — ID готового Prometheus в том же VPC;
- `K2_ACC_ELK_SECURITY_GROUP_ID` — ID security group этого VPC;
- `K2_ACC_ELK_SUBNET_IDS` — ID одной или нескольких subnet через запятую;
- стандартные credentials, region и endpoints провайдера для K2 Cloud.

Harness использует первый ID из `K2_ACC_ELK_SUBNET_IDS`:

```bash
test -n "$K2_ACC_ELK_MONITOR_SERVICE_ID"
test -n "$K2_ACC_ELK_SECURITY_GROUP_ID"
test -n "$K2_ACC_ELK_SUBNET_IDS"

TF_ACC=1 go test ./internal/service/paas \
  -run '^TestAccPaaSELK_parameters$' \
  -count=1 -v -timeout 45m
```

Оба теста используют `ErrorCheck` провайдера и `CheckDestroy`; пропущенный
cleanup считается падением, а не успешным завершением.

## Финальный destroy

```bash
final_elk_id="$(terraform output -raw elk_id)"
final_pipeline_id="$(terraform output -raw pipeline_id)"

terraform destroy \
  -target=aws_paas_logstash_pipeline.qa \
  -auto-approve

elk_id="$final_elk_id"
pipeline_id="$final_pipeline_id"
wait_for_pipeline_absent

terraform destroy -auto-approve
test -z "$(terraform state list)"

set +e
final_service_output="$(
  c2-paas DescribeService serviceId "$final_elk_id" 2>&1
)"
final_service_status=$?
set -e

if [ "$final_service_status" -eq 0 ]; then
  jq -e '.service.status == "DELETED"' \
    <<<"$final_service_output"
else
  grep -Eiq 'ServiceNotFound|not.?found|404' \
    <<<"$final_service_output"
fi

rm -f terraform.tfplan
trap - EXIT INT TERM
unset \
  TF_VAR_elk_password \
  TF_VAR_elk_options \
  K2_QA_ELK_PASSWORD \
  AWS_ACCESS_KEY_ID \
  AWS_SECRET_ACCESS_KEY

cd /
rm -rf -- "$qa_work_dir"
```

Pipeline сначала независимо подтверждается отсутствующим при `READY` родителя.
После полного destroy API обязан вернуть для ELK `DELETED` либо явный
`ServiceNotFound`/404, а Terraform state должен быть пустым. Только после этих
проверок удаляется одноразовый каталог со state и backup-файлами. В отчёт
прикладываются только отфильтрованные результаты `jq`, service/pipeline IDs,
timestamps операций и скриншоты статусов без credentials, пароля, pipeline
configuration и полного Terraform state.
