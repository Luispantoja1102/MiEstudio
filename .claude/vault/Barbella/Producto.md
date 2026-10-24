# Producto

## Planes (fuente: `Backend/go/internal/api/plans.go`; la plataforma puede editarlos desde su panel)

| Plan | Mes | Año | Personas | Sucursales | Caja y ventas | Anticipo transferencia | Anticipo Mercado Pago | Terminal |
|---|---|---|---|---|---|---|---|---|
| Individual | $150 | $1,500 | 1 | 1 | No | Sí | No | No |
| Básico | $350 | $3,500 | 2 | 1 | No | Sí | No | No |
| Crecimiento | $700 | $7,000 | 6 | 1 | Sí | Sí | Sí | Sí |
| Pro | $1,200 | $12,000 | sin límite | 10 | Sí | Sí | Sí | Sí |

- Prueba gratis de 14 días, sin tarjeta. El año cobra 10 meses.
- "Caja y ventas" incluye paquetes, inventario, comisiones y reportes.
- El soporte (correo, prioritario, dedicado) es solo una etiqueta, no hay sistema de tickets.

## Anticipos
- **Transferencia:** el cliente transfiere y sube comprobante (captura, PDF o archivo) o lo manda por WhatsApp. **Solo el dueño** confirma la cita. El dueño captura banco (lista), CLABE y/o tarjeta.
- **Mercado Pago:** se confirma solo, el sistema consulta a Mercado Pago el monto exacto (no se puede falsificar).
- El horario se aparta mientras llega el comprobante (12 h por defecto, 48 h tras enviarlo).

## Agenda
- Vistas día, semana, mes y lista; filtro por persona; bloquear horarios.
- "Llegó" solo el mismo día y hasta 1 hora antes.
- Reserva pública: calendario mensual; lo ocupado se muestra solo como "Ocupado".
