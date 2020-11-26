### ot-webserver

Este repositorio es un ejemplo de instrumentación de observabilidad usando [OpenTelemetry.io](https://opentelemetry.io/) Los requisitos para poder ejecutar este demo son

* Memcache 
* Go (se ha probado con Go 1.15)
* Un compilador de C, para usar sqlite

Este repositorio fué creado como demo para el meetup de Gophers MX, la presentación la pueden consultar [aquí]().

En este repositorio encontrarán casos de instrumentación para [gorm.io](https://gorm.io/), net/http tanto en cliente como servidor y [memcache](github.com/bradfitz/gomemcache).

Se han creado referencias para distintos exporters de trazas, que se encuentran disponibles en distintas ramas del proyecto:

* develop: Exporta las trazas a [honeycomb.io](https://honeycomb.io)

* feature/gcloud: Exporta las trazas a [Google Cloud Trace](https://cloud.google.com/trace/docs/setup/go-ot)

* feature/lightstep: Exporta las trazas a [Lightstep](http://lightstep.com/)


Si cuentas con una cuenta de Honeycomb.io, puedes probar el servicio desde Docker:

> docker run --rm -e LS_ACCESS_TOKEN=<TUKEY> docker.io/wraven/opentelemetry-webserver:lightstep

Lo cual iniciará dentro del contenedor memcached, el servidor web y lo invocará a través del cliente instrumentado con telemetría.
