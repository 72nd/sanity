# Sanity – Ein Überblick

## Überblick

Das Ziel: Eine simple Dokumentenverwaltung für das Einsortieren (richtiger Ordner und Dateinamen) von Belegen in Form von PDF-Dateien in meine Ordnerstruktur. Pro Dokumenttyp gibt es einen Zielpfad mit Platzhalter im Pfad (z.B. für Datum des Dokuments). Die Erkennung von Dokumententyp und Datum sollte möglichst automatisch gehen (Vorschläge an den User) ohne aber auf externe Dienste zu setzen.

Technisch: Die Idee von Sanity ist hauptsächlich, das neue Fine UI-Toolkit anhand einer kleiner Applikation zu testen. UI, Code, Variabeln und Doccoments sollen alle auf Englisch sein.


## Zu UI Entscheidungen

Grundsätzlich sind die Guidlines des Fine Toolkits zu beachten. Layout und UI Entscheidungen sind zu reflektieren. Die verschiedenen Optionen sind gegeneinander Abzuwägen und im Hinblick auf den Einsatzzweck optimiert zu wählen bzw. kombinieren.


## Ziele

Oberstes Ziel ist eine möglichst hohe Beschleunigung beim Einsortieren von Belegen in meiner Buchhaltung. Alles und wirklich alles untersteht diesem Ziel. Geschwindigkeit im Alltag. Helfer und Strukturen die dabei unterstützen und Dinge noch schneller machen, sind zu implementieren.


### Welcher Zweck

Für meine Buchhaltung muss ich immer wieder Dokumente einsortieren. Für jede Buchhaltung (für verschiedene Unternehmen) existiert ein Hauptordner, worin sich alle Belege usw. befinden. Das ist ein Projekt. Die Buchhaltungsdatei ist jeweils in `PRJ/office/` abgelegt. Für jedes Projekt soll eine eigene Sanity Datei (JSON) existieren, worin alles abgespeichert ist.

Für jedes Projekt kann eine Reihe an Belegtypen definiert werden. Jeder Belegtyp verfügt über eine spezifischen Pfad mit Platzhalter, wo die Datei abgespeichert werden soll. Darüber hinaus ist für jeden Belegtyp definiert, mit welchen Heuristiken er automatisch erkannt werden kann.

Ein wichtiger Punkt ist sind die Heuristiken. Für eingespielte Belege sollten möglichst automatisch erkannt werden, um welchen Belegtyp und Dokumentdatum es sich handelt.

Die UI soll für einen möglichst schnellen Workflow designed sein. Usereingaben, Automatiken und Usereingaben sollen flüssig und Hand in Hand gehen.


### Projektstruktur / Projektstruktur

Ein Projekt umfasst ein Dateibestand für ein Unternehmen. Für jedes Projekt soll der Rootpath gespeichert werden, sowie ein Projektname. 

Für jedes Projekt kann der User eine Reihe von Belegtypen (BT) definieren. Jeder BT hat einen Namen, alle notwendigen Informationen und Konfigurationen für die Heuristiken und einen Pfad, wohin ein eingelesenes Dokument kopiert und benannt werden soll.

Dieser Pfad ist jeweils relativ zum Projektordner und kann Platzhalter enthalten. Stand jetzt sind vor allem das Datum wichtig. Der Nutzer soll selbst in der Lage sein im Benutzername einzeln Jahr, Monat und Tag auszuwählen. Dabei sollen verschiedene Datetime Formate unterstützt sein. Es ist ok, dafür die Go-Fromatdatestring-Syntax zu nehmen, solange diese in der UI in aller Kürze erklärt wird und stets ein ausgewerteter Pfad mit einem Beispieldatum da steht. Der Nutzer soll beim Konfigurieren die Méglichkeit haben, mittels der Ordnerauswahl des OS den Pfad auch zu bestimmen.

Bearbeitete Dateien sollen weiterhin in einer Tabelle gesammelt werden. So entsteht über die Zeit eine Dokumentenbibliothek. Dies soll auch die Hauptansicht sein.


### Die UI

Wie oben bereits mehrfach ausgeführt, ist das maximale Beschleunigen meiner Belegeinsortierung das oberste Ziel. Gerade der UI kommt im Erreichen des Ziels die absolut größte Bedeutung zu. Die UI hat das maximal beschleunigte Abarbeiten von neuen PDF Dokumenten zum Ziel. Hierfür soll der Nutzer möglichst effizient mit dem Programm interagieren können. Besonders eine lückenlose Bedienung mit der Tastatur ist deshalb absolut zentral.

Der User soll auf verschiedenen einfachen Wegen eine PDF zur Software hinzufügen.

Vorgeschlagene Einsortierung und Daten sollen Hand in Hand mit Userinterkationen abgebildet werden. Es ist von größter Wichtigkeit, dass besonders das Datumsfeld extrem effizient ausgefüllt werden kann.

Extrem wichtig ist auch die extrem schnelle und gut durchdachte Auswahl des BT durch den User. Also Durchsuchen der vorhandenen BT deren Auswahl etc. alles mit der Tastatur.

Diese User-Story muss makellos mit einem Opus Agent vorgeplant werden. Werden zusätzliche Aspekte erkannt, die zur Schnelligkeit der Bearbeitung beitragen, aber hier nicht in diesem Draft stehen, so sind diese vorzuschlagen.

Der User soll direkt in der Software beim Einsortieren das Dokument sehen können. Also den Inhalt.


### Erkennung, Heuristic

Um die Erkennung zu beschleunigen sollten BTs und das Belegdatum aus den PDFs möglichst automatisiert erkannt werden. Bitte überlege dir, was hierfür am besten geeignet ist. Es dürfen keine externe Dienste oder externe LLMs dafür genutzt werden. Was ist hier möglich?

Es kann davon ausgegangen werden, dass die Software nur auf macOS läuft. Die Machine Learning Frameworks für OCR usw. können also gerne benutzt werden. (Es muss aber geklärt werden, wie diese am besten angesprochen werden können).

Denke wirklich gut durch, was hier alles möglich ist.


## Umsetzung

Für die Umsetzung von Plänen sollten nach Möglichkeit mehrere Agenten (Sonnet und Haiku) verwendet werden.
