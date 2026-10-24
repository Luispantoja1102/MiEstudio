# Anticipos por método y plan

- **Decisión:** `Deposits` = anticipo por transferencia; `DepositsMP` = anticipo con Mercado Pago. Individual y Básico solo transferencia; Crecimiento y Pro ambos.
- **Dónde:** `plans.go`, `transfer.go` (`depositMethods`), `booking_settings.go`.
- **Nota:** un negocio con Mercado Pago conectado en plan Básico no lo ofrece a sus clientes.
