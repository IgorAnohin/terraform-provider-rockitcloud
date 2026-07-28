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
3. Security group разрешает необходимый QA-трафик к Elasticsearch, Kibana и
   тестовому HTTP input Logstash только из доверенной сети.
4. Локально собранный provider подключён через Terraform CLI
   `provider_installation.dev_overrides`.
5. Установлены `terraform`, `jq`, `curl` и `c2-paas`.
6. Следующие переменные окружения содержат реальные значения выделенного стенда:

```bash
set -euo pipefail

: "${AWS_ACCESS_KEY_ID:?AWS_ACCESS_KEY_ID is required}"
: "${AWS_SECRET_ACCESS_KEY:?AWS_SECRET_ACCESS_KEY is required}"
: "${K2_QA_ELK_PASSWORD:?K2_QA_ELK_PASSWORD is required}"
: "${K2_QA_SSH_KEY_NAME:?K2_QA_SSH_KEY_NAME is required}"
: "${K2_QA_SUBNET_IDS_JSON:?K2_QA_SUBNET_IDS_JSON is required}"
: "${K2_QA_SECURITY_GROUP_IDS_JSON:?K2_QA_SECURITY_GROUP_IDS_JSON is required}"

export TF_VAR_region=ru-msk
export TF_VAR_elk_password="$K2_QA_ELK_PASSWORD"
export TF_VAR_ssh_key_name="$K2_QA_SSH_KEY_NAME"
export TF_VAR_subnet_ids="$K2_QA_SUBNET_IDS_JSON"
export TF_VAR_security_group_ids="$K2_QA_SECURITY_GROUP_IDS_JSON"
trap 'rm -f terraform.tfplan' EXIT
```

`K2_QA_SUBNET_IDS_JSON` и `K2_QA_SECURITY_GROUP_IDS_JSON` должны быть JSON-массивами
строк. Секреты не сохраняются в `terraform.tfvars` и не прикладываются к отчёту.
Все команды ниже используют игнорируемый Git-файл `terraform.tfplan` и удаляют
его после применения. Нельзя сохранять или прикладывать сырой вывод
`terraform show -json`: sensitive-значения в JSON не маскируются.

## P0: статическая проверка

Из каталога `examples/paas-elk`:

```bash
terraform fmt -check
terraform init
terraform validate
```

Ожидается успешная инициализация и валидация без обращения к PaaS Create API.

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
    and .data_volume[0].size == 32
    and ([.instances[] | select(.status == "READY")] | length) == 1
    and (.endpoints | length) > 0
'

terraform show -json | jq -e '
  .values.root_module.resources[]
  | select(.address == "aws_paas_logstash_pipeline.qa")
  | .values
  | (.pipeline_id | length) > 0
    and .name == "terraform-qa"
'
```

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
`terraform-qa` счётчик Events In должен увеличиться; в QA-отчёт записывается
только `event_id`, без содержимого state и pipeline configuration.

## P0: replacement при rename

```bash
export TF_VAR_pipeline_name=terraform-qa-renamed
terraform plan -out=terraform.tfplan
terraform show -json terraform.tfplan | jq -e '
  .resource_changes[]
  | select(.address == "aws_paas_logstash_pipeline.qa")
  | .change.actions == ["delete", "create"]
'
rm -f terraform.tfplan
export TF_VAR_pipeline_name=terraform-qa
```

Apply не нужен. Имя отсутствует в Modify API, поэтому plan обязан показать
replacement.

## P0: import pipeline

```bash
elk_id="$(terraform output -raw elk_id)"
pipeline_id="$(terraform output -raw pipeline_id)"

terraform state rm aws_paas_logstash_pipeline.qa
terraform import aws_paas_logstash_pipeline.qa "$elk_id/$pipeline_id"
terraform plan -detailed-exitcode
```

Ожидается восстановление `service_id`, `pipeline_id`, `name` и `configuration`;
повторный plan имеет exit code `0`.

Отдельно проверить ошибки формата:

```bash
for invalid_id in \
  "$elk_id" \
  "/$pipeline_id" \
  "$elk_id/" \
  "$elk_id/$pipeline_id/extra"
do
  terraform state rm aws_paas_logstash_pipeline.qa
  if terraform import aws_paas_logstash_pipeline.qa "$invalid_id"; then
    exit 1
  fi
  terraform import aws_paas_logstash_pipeline.qa "$elk_id/$pipeline_id"
done
```

Каждая команда должна завершиться ошибкой с форматом
`service_id/pipeline_id` и не менять state.

## P0: import ELK service

Проверить унаследованный generic import без изменения облачного сервиса:

```bash
elk_id="$(terraform output -raw elk_id)"

terraform state rm aws_paas_service.elk
terraform import aws_paas_service.elk "$elk_id"
test "$elk_id" = "$(terraform output -raw elk_id)"
terraform plan -detailed-exitcode
```

Import должен выбрать manager по полученному из API `serviceType = elk`,
восстановить ELK block и завершиться пустым plan. Если API не возвращает
write-only пароль и plan предлагает replacement, apply выполнять нельзя: это
release blocker, который фиксируется в отчёте.

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
  if "$@"; then
    echo "terraform plan unexpectedly succeeded: $*" >&2
    return 1
  fi

  return 0
}
```

| Случай | Команда или изменение | Ожидаемый результат |
|---|---|---|
| Пустая версия | `expect_plan_failure env TF_VAR_elk_version= terraform plan` | Ошибка до API |
| Пароль короче 8 символов | `expect_plan_failure env TF_VAR_elk_password=Abcd123 terraform plan` | Ошибка до API |
| Запрещённый символ в пароле | `expect_plan_failure env TF_VAR_elk_password='Abcd!234' terraform plan` | Ошибка до API |
| Неизвестная anonymous role | `expect_plan_failure env TF_VAR_allow_anonymous=true TF_VAR_anonymous_roles='["admin"]' terraform plan` | Ошибка до API |
| Зарезервированное имя pipeline | `expect_plan_failure env TF_VAR_pipeline_name=beats-to-elasticsearch terraform plan` | Ошибка до API |
| Пустая configuration | `expect_plan_failure env TF_VAR_pipeline_configuration= terraform plan` | Ошибка до API |
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
if TF_VAR_pipeline_configuration='input {' terraform apply -auto-approve; then
  exit 1
fi
test "$pipeline_id_before" = "$(terraform output -raw pipeline_id)"
terraform plan -detailed-exitcode
```

Apply должен завершиться понятной ошибкой PaaS. ID и сохранённая сервером
configuration остаются прежними; после возврата к валидному значению повторный
plan имеет exit code `0`.

Проверка не-ELK parent изолирована отдельным ресурсом и не меняет основной
pipeline:

```bash
: "${K2_QA_NON_ELK_SERVICE_ID:?K2_QA_NON_ELK_SERVICE_ID is required}"
if TF_VAR_wrong_service_id="$K2_QA_NON_ELK_SERVICE_ID" \
  terraform apply \
    -target='aws_paas_logstash_pipeline.wrong_service[0]' \
    -auto-approve
then
  exit 1
fi
if terraform state list |
  grep -Fxq 'aws_paas_logstash_pipeline.wrong_service[0]'
then
  exit 1
fi
terraform plan -detailed-exitcode
```

Ожидается ошибка API о несовместимом типе сервиса и отсутствие временного
pipeline в state.

## P1: anonymous access

Поля анонимного доступа replacement-only, поэтому сценарий выполняется после
основных pipeline-проверок:

```bash
elk_id_before="$(terraform output -raw elk_id)"
export TF_VAR_allow_anonymous=true
export TF_VAR_anonymous_roles='["viewer","editor"]'

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
    and (.service.parameters.anonymousRole | sort) == ["editor", "viewer"]
'
terraform plan -detailed-exitcode

export TF_VAR_anonymous_roles='["editor","viewer"]'
terraform plan -detailed-exitcode
```

Ожидается массив из двух ролей в API, обе проверки plan имеют exit code `0`, а
перестановка ролей не создаёт diff. В UI проверить доступ к Kibana и роли
`viewer`/`editor`. Поле `options` проверяется unit-тестами преобразования:
публичная документация не публикует допустимые version-specific ключи, поэтому
manual QA не должен придумывать непроверенный ключ.

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
terraform plan -detailed-exitcode
```

В UI/API дополнительно проверить роль arbitrator и отсутствие у него прикладных
Kibana/Logstash endpoints. Повторный plan должен иметь exit code `0`.

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
`READY`, data source отражает актуальные labels.

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

## Финальный destroy

```bash
terraform destroy -auto-approve
terraform show
rm -f terraform.tfplan
```

Ожидается удаление pipeline до родительского ELK по зависимости `service_id`,
удаление сервиса до `DELETED`/NotFound и пустой Terraform state. В отчёт приложить
только отфильтрованные результаты `jq`, service/pipeline IDs, timestamps операций
и скриншоты статусов без credentials, пароля, pipeline configuration и полного
Terraform state.
