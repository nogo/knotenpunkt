# Invented claims for the synthetic vault. Each page: 15 live claims.
# Entry: (key, text, trust, party_or_None, attributed)
S, C, U, P = "sourced", "committed", "unverified", "position"
PAGES = {
"ACME Migration": [
 ("am01","The ACME kickoff notes state that the warehouse system migration starts on 2026-11-03 and must finish before the Christmas freeze begins on 2026-12-15.",S,None,True),
 ("am02","According to the ACME steering minutes, the migration budget is capped at 420,000 EUR, and any overrun above five percent needs approval from the steering board.",S,None,True),
 ("am03","Jane Roe committed in the steering meeting that ACME's operations team provides two dedicated testers for the whole migration, starting in the first sprint.",C,"Jane Roe",False),
 ("am04","The ACME architecture overview states that the legacy warehouse system runs on an on-premise Oracle 12c database that is out of vendor support since 2025.",S,None,True),
 ("am05","The migration plan states that ACME moves its three regional warehouses one at a time, beginning with the Hamburg site because it has the lowest order volume.",S,None,True),
 ("am06","According to the risk register, the biggest migration risk for ACME is the barcode scanner firmware, which only speaks a proprietary protocol to the old system.",S,None,True),
 ("am07","The ACME kickoff notes state that the cutover of each warehouse happens on a Sunday night, with a maximum planned downtime of six hours per site.",S,None,True),
 ("am08","John Doe committed to deliver the data mapping for all 214 legacy tables of the ACME warehouse system by the end of the second sprint.",C,"John Doe",False),
 ("am09","The ACME test strategy states that every cutover rehearsal uses a full copy of production data, anonymised for customer names and delivery addresses.",S,None,True),
 ("am10","According to the steering minutes, ACME decided against a big-bang cutover after the failed ERP rollout in 2023 stopped shipping for three days.",S,None,True),
 ("am11","The migration plan states that the old warehouse system stays available read-only for twelve months after the last site has moved, for audits and returns.",S,None,True),
 ("am12","I think the Hamburg-first sequence underestimates the scanner risk, because Hamburg uses the oldest scanner models of all three ACME sites.",P,None,False),
 ("am13","The ACME project charter states that the migration is sponsored by the COO and that the CIO only has an advisory role in the steering board.",S,None,True),
 ("am14","It is believed that ACME's Rotterdam warehouse plans to switch to a different shipping carrier during the migration, but no document confirms this yet.",U,None,False),
 ("am15","According to the sprint review notes, the ACME team works in two-week sprints and holds the sprint review every second Thursday with the operations leads.",S,None,True),
],
"ACME Warehouse API": [
 ("aw01","The ACME API specification states that the new warehouse API is REST over HTTPS with JSON payloads, versioned in the URL path starting with /v1.",S,None,True),
 ("aw02","According to the API specification, every request to the ACME warehouse API needs an OAuth 2 client-credentials token issued by ACME's central identity provider.",S,None,True),
 ("aw03","The ACME integration notes state that the warehouse API must answer stock queries within 300 milliseconds at the 95th percentile under peak load.",S,None,True),
 ("aw04","The API specification states that the ACME warehouse API publishes stock changes as events on a Kafka topic named warehouse.stock.v1.",S,None,True),
 ("aw05","Max Mustermann committed that ACME's platform team operates the Kafka cluster for the warehouse events and guarantees a seven-day retention.",C,"Max Mustermann",False),
 ("aw06","According to the integration notes, the ACME webshop calls the warehouse API for stock checks only, never for reservations, which stay in the order system.",S,None,True),
 ("aw07","The API specification states that the ACME warehouse API rate-limits each client to 50 requests per second and returns HTTP 429 when the limit is reached.",S,None,True),
 ("aw08","The ACME security review states that the warehouse API must not expose customer personal data; shipments are referenced only by an opaque shipment id.",S,None,True),
 ("aw09","According to the integration notes, ACME's carrier partners receive shipment updates by webhook, signed with an HMAC-SHA256 signature per partner.",S,None,True),
 ("aw10","The API specification states that bulk stock imports go through a separate batch endpoint that accepts CSV files of at most 100,000 lines.",S,None,True),
 ("aw11","The ACME integration notes state that the old SOAP interface for stock queries is switched off three months after the last warehouse cutover.",S,None,True),
 ("aw12","According to the security review, all warehouse API traffic between ACME data centres is encrypted with mutual TLS using certificates from the internal CA.",S,None,True),
 ("aw13","I prefer a pull-based stock sync for the webshop over the Kafka events, because the webshop team has no experience operating Kafka consumers.",P,None,False),
 ("aw14","The API specification states that the ACME warehouse API returns all timestamps in UTC using ISO 8601 format with a trailing Z.",S,None,True),
 ("aw15","According to the integration notes, the warehouse API's sandbox environment is refreshed with anonymised data every Monday morning at 06:00.",S,None,True),
],
"OrderApp Checkout": [
 ("oc01","The OrderApp product brief states that the new checkout must support guest orders without an account, because 38 percent of orders today are guest orders.",S,None,True),
 ("oc02","According to the OrderApp analytics report, the current checkout loses 61 percent of mobile users between the cart and the payment step.",S,None,True),
 ("oc03","The OrderApp product brief states that the redesigned checkout goes live for all markets on 2027-02-01, after a four-week pilot in Austria.",S,None,True),
 ("oc04","Jane Roe committed that OrderApp's design team delivers the final checkout screens for mobile and desktop before the end of November 2026.",C,"Jane Roe",False),
 ("oc05","The OrderApp architecture notes state that the checkout is rebuilt as a server-rendered web application, not as a native mobile app.",S,None,True),
 ("oc06","According to the product brief, the new OrderApp checkout reduces the number of form fields for a guest order from 19 to 9.",S,None,True),
 ("oc07","The OrderApp accessibility audit states that the current checkout fails WCAG 2.1 AA on colour contrast and keyboard navigation of the address form.",S,None,True),
 ("oc08","The OrderApp architecture notes state that the checkout validates addresses through an external address service and falls back to free text on timeout.",S,None,True),
 ("oc09","According to the product brief, OrderApp keeps the cart for logged-in users for 30 days, and for guest users for 24 hours in a browser cookie.",S,None,True),
 ("oc10","The OrderApp analytics report states that customers in Germany prefer invoice payment, which accounts for 41 percent of completed orders there.",S,None,True),
 ("oc11","According to the steering minutes, OrderApp decided that the new checkout ships without a loyalty points feature, which comes in a later release.",S,None,True),
 ("oc12","I believe the 2027-02-01 go-live is too ambitious for OrderApp, because the payment provider contract is not signed yet.",P,None,False),
 ("oc13","The OrderApp test plan states that the checkout is tested on the five most used mobile browsers, which together cover 93 percent of mobile traffic.",S,None,True),
 ("oc14","It is believed that OrderApp's marketing team wants a one-click reorder button in the checkout, but no requirement document mentions it yet.",U,None,False),
 ("oc15","The OrderApp architecture notes state that checkout sessions are stored in Redis with a sliding expiry of 45 minutes of inactivity.",S,None,True),
],
"OrderApp Payments": [
 ("op01","The OrderApp payment concept states that card payments, PayPal and invoice are the three payment methods supported at go-live in every market.",S,None,True),
 ("op02","According to the payment concept, OrderApp uses a single payment service provider for cards and PayPal to keep reconciliation in one system.",S,None,True),
 ("op03","The OrderApp payment concept states that invoice payment is offered only to customers who pass a credit check with a score of at least 600.",S,None,True),
 ("op04","John Doe committed that OrderApp's finance team signs the payment provider contract by 2026-12-01 at the latest.",C,"John Doe",False),
 ("op05","The OrderApp security notes state that the shop never stores full card numbers; card data goes directly to the provider's hosted fields.",S,None,True),
 ("op06","According to the payment concept, OrderApp captures card payments at shipment, not at order time, so a cancelled order needs no refund.",S,None,True),
 ("op07","The OrderApp payment concept states that refunds above 500 EUR need a second approval by a finance team member before they are released.",S,None,True),
 ("op08","According to the reconciliation notes, OrderApp reconciles provider settlements with orders once per day in a batch job that runs at 02:00.",S,None,True),
 ("op09","The OrderApp security notes state that the payment integration must pass a PCI DSS SAQ A self-assessment before go-live.",S,None,True),
 ("op10","According to the payment concept, OrderApp charges no fee for invoice payment, and the payment term for invoices is 14 days.",S,None,True),
 ("op11","The OrderApp reconciliation notes state that chargebacks are handled manually by two people in the finance team, using the provider's dashboard.",S,None,True),
 ("op12","I think OrderApp should drop invoice payment in Austria, because its default rate there was twice the German rate last year.",P,None,False),
 ("op13","The payment concept states that OrderApp shows prices including VAT in all markets and calculates VAT per line item, not per order.",S,None,True),
 ("op14","According to the analytics report, PayPal accounts for 27 percent of OrderApp's completed orders across all markets.",S,None,True),
 ("op15","The OrderApp payment concept states that failed card payments are retried once after 24 hours before the order is cancelled automatically.",S,None,True),
],
"Globex Payroll": [
 ("gp01","Laut Projektauftrag von Globex verbindet die Payroll-Integration das neue HR-System mit dem externen Abrechnungsdienstleister für 2.300 Mitarbeitende in vier Ländern.",S,None,True),
 ("gp02","Laut Schnittstellenspezifikation von Globex werden die Stammdaten der Mitarbeitenden täglich um 22:00 Uhr Ortszeit als Delta-Datei an den Abrechnungsdienstleister übertragen.",S,None,True),
 ("gp03","Der Projektauftrag von Globex legt fest, dass die Payroll-Integration vor dem Abrechnungslauf Januar 2027 live sein muss, der am 2027-01-20 abgeschlossen wird.",S,None,True),
 ("gp04","Max Mustermann hat zugesagt, dass Globex HR alle doppelten Personaldatensätze im HR-System vor dem ersten Testabrechnungslauf bereinigt.",C,"Max Mustermann",False),
 ("gp05","Laut Schnittstellenspezifikation sendet der Abrechnungsdienstleister die Entgeltabrechnungen als PDF-Dateien und eine CSV-Zusammenfassung pro Abrechnungsperiode an Globex zurück.",S,None,True),
 ("gp06","Laut Testplan von Globex müssen zwei Parallelläufe im alten und neuen Prozess centgenau übereinstimmen, bevor der alte Prozess abgeschaltet wird.",S,None,True),
 ("gp07","Die Schnittstellenspezifikation legt fest, dass Globex Dateien an den Abrechnungsdienstleister ausschließlich per SFTP mit schlüsselbasierter Anmeldung überträgt.",S,None,True),
 ("gp08","Laut Projektauftrag von Globex kommt die spanische Gesellschaft erst in einer zweiten Phase zur Payroll-Integration, nachdem die anderen drei Länder live sind.",S,None,True),
 ("gp09","Laut Lenkungskreisprotokoll behält Globex die von Führungskräften genehmigten Gehaltsänderungen im HR-System und überträgt nur die genehmigten Werte an die Abrechnung.",S,None,True),
 ("gp10","Die Schnittstellenspezifikation von Globex legt fest, dass Mitarbeitende gegenüber dem Abrechnungsdienstleister über eine stabile Personalnummer identifiziert werden, nie über den Namen.",S,None,True),
 ("gp11","Laut Testplan von Globex nutzt der erste Testabrechnungslauf die Oktober-2026-Daten von 150 Freiwilligen der deutschen Gesellschaft.",S,None,True),
 ("gp12","Ich finde, Globex sollte die spanische Gesellschaft in die erste Phase ziehen, weil ihr Vertrag mit dem Abrechnungsdienstleister im März 2027 endet.",P,None,False),
 ("gp13","Laut Lenkungskreisprotokoll berichtet das Payroll-Integrationsprojekt von Globex an den CFO, fachlich verantwortlich ist die HR-Leitung.",S,None,True),
 ("gp14","Vermutlich plant Globex, den Abrechnungsdienstleister in Österreich im nächsten Jahr zu wechseln, aber der Einkauf hat das nicht bestätigt.",U,None,False),
 ("gp15","Laut Schnittstellenspezifikation stoppt eine Delta-Datei, die die Validierung nicht besteht, die gesamte Übertragung, und das HR-Operations-Team wird per E-Mail alarmiert.",S,None,True),
],
"Globex Data Protection": [
 ("gd01","Laut Datenschutz-Folgenabschätzung von Globex sind Abrechnungsdaten wegen gesundheitsbezogener Fehlzeiten besondere Kategorien personenbezogener Daten und müssen verschlüsselt gespeichert werden.",S,None,True),
 ("gd02","Laut Datenschutz-Folgenabschätzung von Globex werden Entgeltabrechnungs-PDFs 30 Tage nach Zustellung an die Mitarbeitenden vom Integrationsserver gelöscht.",S,None,True),
 ("gd03","Die Betriebsvereinbarung von Globex legt fest, dass das HR-System nicht genutzt werden darf, um die Leistung einzelner Mitarbeitender anhand von Abrechnungsdaten zu bewerten.",S,None,True),
 ("gd04","Jane Roe hat zugesagt, dass die Datenschutzbeauftragte von Globex die finale Schnittstellenspezifikation prüft, bevor Produktivdaten übertragen werden.",C,"Jane Roe",False),
 ("gd05","Laut Datenschutz-Folgenabschätzung verarbeitet der externe Abrechnungsdienstleister die Daten von Globex ausschließlich in Rechenzentren innerhalb der Europäischen Union.",S,None,True),
 ("gd06","Laut Berechtigungskonzept von Globex dürfen nur fünf namentlich benannte Personen in HR Operations den Inhalt der Abrechnungsdateien einsehen.",S,None,True),
 ("gd07","Das Berechtigungskonzept von Globex legt fest, dass Zugriffsrechte auf Abrechnungsdaten quartalsweise von der HR-Leitung überprüft und im Ticketsystem dokumentiert werden.",S,None,True),
 ("gd08","Laut Datenschutz-Folgenabschätzung müssen Testabrechnungsläufe pseudonymisierte Gehaltsdaten verwenden, sofern die Mitarbeitenden nicht schriftlich eingewilligt haben.",S,None,True),
 ("gd09","Laut Betriebsvereinbarung informiert Globex den Betriebsrat mindestens vier Wochen bevor eine Änderung am Abrechnungsdatenfluss live geht.",S,None,True),
 ("gd10","Der Auftragsverarbeitungsvertrag von Globex legt fest, dass der Abrechnungsdienstleister eine Datenschutzverletzung innerhalb von 24 Stunden nach Kenntnis an Globex meldet.",S,None,True),
 ("gd11","Laut Berechtigungskonzept protokolliert Globex jeden Lesezugriff auf Abrechnungsdateien und bewahrt diese Protokolle zwei Jahre auf.",S,None,True),
 ("gd12","Ich halte die quartalsweise Rechteprüfung bei Globex für zu langsam, weil sich das Personal in HR Operations fast jeden Monat ändert.",P,None,False),
 ("gd13","Laut Datenschutz-Folgenabschätzung werden Backups des Integrationsservers verschlüsselt und 90 Tage in einem zweiten Rechenzentrum aufbewahrt.",S,None,True),
 ("gd14","Vermutlich möchte der Betriebsrat von Globex die Liste der Unterauftragnehmer des Abrechnungsdienstleisters sehen, aber eine Anfrage ist nicht dokumentiert.",U,None,False),
 ("gd15","Der Auftragsverarbeitungsvertrag von Globex legt fest, dass der Abrechnungsdienstleister Unterauftragnehmer nur nach schriftlicher Genehmigung durch Globex einsetzen darf.",S,None,True),
],
}

# Confirmed corrections already in the log: (key, supersedes, text, trust, party, attributed)
CORRECTIONS = [
 ("am02x","am02","According to the revised ACME steering minutes, the migration budget is raised to 465,000 EUR after adding the scanner firmware work, and overruns above five percent still need board approval.",S,None,True),
 ("aw03x","aw03","The updated ACME integration notes state that the warehouse API must answer stock queries within 200 milliseconds at the 95th percentile under peak load.",S,None,True),
 ("oc03x","oc03","The revised OrderApp product brief states that the redesigned checkout goes live for all markets on 2027-03-01, after a four-week pilot in Austria.",S,None,True),
 ("op10x","op10","According to the updated payment concept, OrderApp charges no fee for invoice payment, and the payment term for invoices is extended to 30 days.",S,None,True),
 ("gp11x","gp11","Laut überarbeitetem Testplan von Globex nutzt der erste Testabrechnungslauf die September-2026-Daten von 150 Freiwilligen der deutschen Gesellschaft.",S,None,True),
 ("gd02x","gd02","Laut überarbeiteter Datenschutz-Folgenabschätzung von Globex werden Entgeltabrechnungs-PDFs 14 Tage nach Zustellung vom Integrationsserver gelöscht.",S,None,True),
]

# Open proposals with known labels against one live claim:
# (key, target, label, kind, text, trust, party, attributed)
PROPOSALS = [
 # ACME Migration
 ("pa1","am01","conflicts","date","The updated ACME project plan states that the warehouse migration now starts on 2027-01-12, after the Christmas freeze has ended.",S,None,True),
 ("pa2","am07","conflicts","number","According to the cutover runbook, each ACME warehouse cutover has a planned downtime of up to twelve hours, spread over a Saturday night.",S,None,True),
 ("pa3","am10","agrees","paraphrase","The ACME lessons-learned page states that the team rejected a single big-bang cutover because the 2023 ERP go-live halted shipping for three days.",S,None,True),
 ("pa4","am05","agrees","detail","According to the cutover runbook, Hamburg is the first ACME warehouse to move, chosen for its low order volume, followed by the other two sites.",S,None,True),
 ("pa5","am03","unrelated","same-topic","According to the ACME test strategy, cutover rehearsals are scheduled for the Friday before each warehouse move and last about four hours.",S,None,True),
 ("pa6","am11","unrelated","same-topic","The ACME migration plan states that returns received after cutover are booked in the new warehouse system from the first day.",S,None,True),
 # ACME Warehouse API
 ("pb1","aw07","conflicts","number","The revised ACME API specification states that each client of the warehouse API may send up to 200 requests per second before it is throttled.",S,None,True),
 ("pb2","aw06","conflicts","negation","According to the new integration notes, the ACME webshop reserves stock directly through the warehouse API when a customer places an order.",S,None,True),
 ("pb3","aw02","agrees","paraphrase","The ACME security review confirms that clients authenticate to the warehouse API with OAuth 2 client-credentials tokens from the central identity provider.",S,None,True),
 ("pb4","aw14","agrees","detail","According to the API changelog, all timestamps in the ACME warehouse API responses are UTC in ISO 8601 format, for example 2026-11-03T22:00:00Z.",S,None,True),
 ("pb5","aw04","unrelated","same-topic","The ACME API specification states that the warehouse API publishes its OpenAPI document at /v1/openapi.json for client code generation.",S,None,True),
 ("pb6","aw10","unrelated","same-topic","According to the integration notes, ACME's carrier partners can query shipment status through a read-only endpoint that requires a partner API key.",S,None,True),
 # OrderApp Checkout
 ("pc1","oc01","conflicts","negation","The revised OrderApp product brief states that the new checkout requires every customer to create an account before placing an order.",S,None,True),
 ("pc2","oc15","conflicts","number","Laut aktualisierten Architekturnotizen laufen Checkout-Sitzungen von OrderApp in Redis nach 15 Minuten Inaktivität ab.",S,None,True),
 ("pc3","oc06","agrees","paraphrase","Das UX-Review von OrderApp hält fest, dass eine Gastbestellung im neuen Checkout nur noch 9 statt bisher 19 Formularfelder braucht.",S,None,True),
 ("pc4","oc05","agrees","detail","According to the OrderApp tech radar, the checkout is built as a server-rendered web application and no native app is planned for the checkout flow.",S,None,True),
 ("pc5","oc09","unrelated","same-topic","The OrderApp product brief states that the checkout shows the expected delivery date before the customer chooses a payment method.",S,None,True),
 ("pc6","oc13","unrelated","same-topic","Laut Testplan von OrderApp simulieren Lasttests vor dem Go-live 2.000 gleichzeitige Checkouts auf der Staging-Umgebung.",S,None,True),
 # OrderApp Payments
 ("pd1","op06","conflicts","timing","Laut überarbeitetem Zahlungskonzept von OrderApp werden Kartenzahlungen sofort bei Bestellung eingezogen, und Stornierungen werden erstattet.",S,None,True),
 ("pd2","op07","conflicts","number","According to the finance policy, OrderApp refunds above 1,000 EUR need a second approval before they are released.",S,None,True),
 ("pd3","op05","agrees","paraphrase","Die PCI-Scoping-Notizen von OrderApp bestätigen, dass vollständige Kartennummern den Shop nie erreichen, weil Kartendaten in den Hosted Fields des Anbieters eingegeben werden.",S,None,True),
 ("pd4","op08","agrees","detail","According to the operations runbook, the OrderApp settlement reconciliation batch runs every night at 02:00 and takes about twenty minutes.",S,None,True),
 ("pd5","op03","unrelated","same-topic","Laut Zahlungskonzept erhalten Kundinnen und Kunden die Rechnung bei Rechnungskauf als PDF im Anhang der Versandbestätigung.",S,None,True),
 ("pd6","op14","unrelated","same-topic","According to the payment concept, OrderApp shows saved PayPal accounts to logged-in customers after they have paid with PayPal once.",S,None,True),
 # Globex Payroll
 ("pe1","gp02","conflicts","timing","Laut überarbeiteter Schnittstellenspezifikation von Globex werden die Stammdaten einmal pro Woche sonntags als Volldatei an den Abrechnungsdienstleister übertragen.",S,None,True),
 ("pe2","gp08","conflicts","scope","According to the updated Globex project charter, the Spanish entity joins the payroll integration in the first phase together with the other three countries.",S,None,True),
 ("pe3","gp10","agrees","paraphrase","Das Feldmapping der Payroll bestätigt, dass Mitarbeitende beim Abrechnungsdienstleister nur über ihre stabile Personalnummer zugeordnet werden, nicht über den Namen.",S,None,True),
 ("pe4","gp07","agrees","detail","According to the Globex infrastructure notes, the SFTP transfer to the payroll provider uses ed25519 keys and password login is disabled.",S,None,True),
 ("pe5","gp05","unrelated","same-topic","Laut Schnittstellenspezifikation bestätigt der Abrechnungsdienstleister jede empfangene Delta-Datei innerhalb einer Stunde mit einer Quittungsdatei.",S,None,True),
 ("pe6","gp13","unrelated","same-topic","According to the Globex steering minutes, the payroll integration steering board meets every four weeks on Tuesday afternoon.",S,None,True),
 # Globex Data Protection
 ("pf1","gd05","conflicts","scope","Laut aktualisiertem Auftragsverarbeitungsvertrag darf der Abrechnungsdienstleister die Daten von Globex auch in einem Supportzentrum in Indien verarbeiten.",S,None,True),
 ("pf2","gd06","conflicts","number","According to the revised Globex access concept, twelve people in HR operations and IT support can see the content of the payroll transfer files.",S,None,True),
 ("pf3","gd15","agrees","paraphrase","Die Einkaufsnotizen von Globex bestätigen, dass der Abrechnungsdienstleister vor dem Einsatz jedes Unterauftragnehmers die schriftliche Genehmigung von Globex braucht.",S,None,True),
 ("pf4","gd11","agrees","detail","According to the Globex security log policy, every read of a payroll transfer file is logged, and the logs are retained for two years in the SIEM.",S,None,True),
 ("pf5","gd09","unrelated","same-topic","Laut Betriebsvereinbarung erhält der Betriebsrat jährlich einen Bericht über die Zahl der HR-System-Nutzer mit Zugriff auf Abrechnungsdaten.",S,None,True),
 ("pf6","gd01","unrelated","same-topic","According to the Globex data protection assessment, employees can request a copy of all payroll data stored about them through the HR portal.",S,None,True),
]

# Each page is one engagement; the org is the client.
NAMESPACES = {
 "ACME Migration": "acme/migration",
 "ACME Warehouse API": "acme/warehouse-api",
 "OrderApp Checkout": "orderapp/checkout",
 "OrderApp Payments": "orderapp/payments",
 "Globex Payroll": "globex/payroll",
 "Globex Data Protection": "globex/data-protection",
}

# Claims true for the whole org, so every engagement of that org reads them.
ORG_LEVEL = {"am13", "aw12", "oc10", "op14", "gd03", "gd09"}

# Proposals that look like a claim of another org. Recall inside the
# proposal's namespace must never return that claim.
# (key, namespace, lookalike, text, trust, party, attributed)
LOOKALIKES = [
 ("xa1","orderapp/checkout","am01","The OrderApp kickoff notes state that the checkout migration starts on 2026-11-03 and must finish before the Christmas freeze begins on 2026-12-15.",S,None,True),
 ("xa2","globex/payroll","am02x","According to the revised Globex steering minutes, the payroll budget is raised to 465,000 EUR, and overruns above five percent still need board approval.",S,None,True),
 ("xa3","acme/warehouse-api","op05","The ACME security notes state that the warehouse API never stores full card numbers; card data goes directly to the provider's hosted fields.",S,None,True),
 ("xa4","acme/migration","gp06","Laut Testplan von ACME müssen zwei Parallelläufe im alten und neuen Lagersystem übereinstimmen, bevor der alte Prozess abgeschaltet wird.",S,None,True),
 ("xa5","orderapp/payments","gd06","According to the OrderApp access concept, only five named people in finance can see the content of the settlement files.",S,None,True),
 ("xa6","globex/data-protection","aw08","Die Sicherheitsprüfung von Globex legt fest, dass die HR-Schnittstelle keine personenbezogenen Daten preisgeben darf; Mitarbeitende werden nur über eine opake Kennung referenziert.",S,None,True),
]
