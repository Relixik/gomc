# 📏 CONVENTIONS — Règles de code & de maintenabilité

But : que le projet reste **facile à maintenir, corriger et améliorer** — notamment parce qu'on suivra un protocole qui change à chaque version de Minecraft. Ces règles sont **appliquées par l'outillage** (CI) autant que possible, pas seulement écrites.

---

## 0. La règle inviolable

> **Zéro dépendance tierce dans le code du serveur.** `go.mod` n'a aucun bloc `require`, `go.sum` reste vide.

C'est le fondement du projet (contrôle total du protocole). Les **outils de dev** (linters, CI) ne comptent pas comme dépendances : ils ne sont pas importés par le code. Avant d'ajouter le moindre import non-stdlib : **stop**, on l'écrit à la main (voir [PLAN.md §4](PLAN.md#4-le--zéro-lib--en-pratique)).

---

## 1. Formatage & outillage (non négociable, vérifié en CI)

| Outil | Rôle | Commande |
|-------|------|----------|
| `gofmt` | formatage canonique | `gofmt -l .` doit être **vide** |
| `goimports` | tri/regroupement des imports | stdlib seule, pas de groupe tiers |
| `go vet` | erreurs statiques | `go vet ./...` |
| `staticcheck` | lint approfondi | `staticcheck ./...` |
| `govulncheck` | CVE (surface faible mais hygiène) | `govulncheck ./...` |
| `go test -race` | tests + détecteur de courses | `go test -race ./...` |

`.editorconfig` et `.golangci.yml` fixent ces règles à la racine. **Aucun merge sans CI verte.**

---

## 2. Nommage & structure

- **Packages** : nom court, minuscule, sans underscore ni pluriel (`codec`, `packet`, `world`). Le nom du dossier = le nom du package.
- **`internal/`** : tout le code non-public y vit (empêche tout import externe). Seul `cmd/server` est un `main`.
- **Layering strict** : une couche n'importe que des couches **inférieures** (voir le tableau [PLAN.md §3](PLAN.md#3-architecture-packages-go)). Aucun cycle d'import. `codec` n'importe rien ; `game/*` n'importe jamais `net/session`.
- **Un `doc.go` par package** décrit sa responsabilité et son périmètre stdlib (déjà en place).
- **Symboles exportés** : commentaire godoc commençant par le nom du symbole (`// Reader lit …`).
- Acronymes en capitales cohérentes : `ID`, `UUID`, `NBT`, `TPS` (pas `Id`, `Uuid`).

---

## 3. Gestion d'erreurs

> Leçon reprise de l'ancien code (commit T002.2 « élimination des panic ») : **un client malveillant ou un paquet malformé ne doit jamais crasher le serveur.**

- **Pas de `panic`** dans le décodage de paquets ni dans une goroutine de connexion. Une erreur de décodage ⇒ on **retourne une erreur** ⇒ la session **déconnecte proprement** ce client, le serveur continue.
- `panic` réservé aux invariants programmeur impossibles (bug logique), jamais à une entrée réseau.
- Envelopper avec `%w` (`fmt.Errorf("decode handshake: %w", err)`) pour garder la chaîne.
- Erreurs sentinelles exportées quand le caller doit discriminer (`var ErrPacketTooLarge = errors.New(...)`).
- Logger via `log/slog` avec contexte (`slog.Warn("decode failed", "state", st, "id", id, "remote", addr, "err", err)`), jamais `fmt.Println`.

---

## 4. Concurrence (les règles qui évitent 90 % des bugs)

> Leçon reprise de l'ancien code (commits T002.1 races / T002.3 fuites de goroutines).

1. **L'état du monde n'est muté que sur la goroutine de tick** (`internal/game/loop`). Conséquence : **aucun mutex** sur les chunks/entités/joueurs.
2. **Inter-goroutines = channels**, pas de mémoire partagée. Connexion → loop : un channel d'intents borné. Loop → connexion : enqueue sur le channel sortant du joueur.
3. **Channels sortants bornés** : un socket lent ne bloque jamais le tick. Overflow ⇒ drop + déconnexion, jamais de blocage.
4. **Cycle de vie via `context.Context`** : chaque connexion a son contexte ; à la fermeture on annule, on draine/ferme le sortant, on poste **un** intent de retrait.
5. **Pas de fuite de goroutine** : toute goroutine lancée a une condition de sortie claire. Test obligatoire : compter les goroutines avant/après N cycles connect-disconnect.
6. `sync` toléré uniquement aux frontières d'enregistrement (ajout/retrait d'un joueur dans la map), pas sur le chemin chaud.

---

## 5. Conventions de paquets (le cœur maintenable)

C'est là que la maintenabilité face au *version churn* se joue.

- **Un fichier par état et direction** : `clientbound_play.go`, `serverbound_login.go`, etc.
- **Chaque paquet** = une struct implémentant `Encode(*codec.Writer)` et/ou `Decode(*codec.Reader)` — **codecs écrits à la main**, pas de réflexion sur tags.
- **IDs en `const` nommées, sourcées et datées** en tête de fichier :
  ```go
  const (
      // Vérifié vs jar serveur 26.1.2 déobfusqué (2026-06-xx).
      idKeepAlive  = 0x1C // serverbound Play
      // 773 wiki, présumé 775 — À CONFIRMER avant de fiabiliser.
      idChatCommand = 0x05
  )
  ```
  Un ID non confirmé porte explicitement le marqueur `présumé` (voir [WORKFLOW.md §3](WORKFLOW.md#3-vérifier-les-ids-play)).
- **Dispatch sur `(State, Direction, ID)`**, jamais sur l'ID seul.
- **Anti-DoS systématique** dans la couche frame : caps VarInt 5/10 octets, longueur 3 octets, paquet 2 097 151 octets — **avant** toute allocation.
- **On ne fait jamais confiance au client** : toute valeur reçue est validée (position plausible, longueur de tableau bornée, enum dans la plage).

---

## 6. Tests

- **Table-driven golden-bytes obligatoire** pour chaque paquet et chaque primitive : `{valeur Go ⇄ octets exacts}`. C'est ce qui fige le format sur le fil et détecte une régression de version.
- Fichiers de vecteurs et captures vanilla dans `testdata/`.
- `go test -race` toujours.
- **Pas de merge d'un jalon sans ses 3 niveaux de vérif** (golden + intégration scriptée + diff de capture vanilla) — voir [WORKFLOW.md §2](WORKFLOW.md#2-le-harnais-de-vérification-protocole-3-niveaux).
- Test de non-fuite de goroutines pour tout ce qui touche aux connexions.

---

## 7. Documentation

- Tout package a un `doc.go`. Tout symbole exporté est documenté (godoc).
- **`PLAN.md` / `WORKFLOW.md` / `CONVENTIONS.md` sont la source de vérité** et doivent rester synchronisés avec le code (mise à jour dans le même commit que le changement qu'ils décrivent).
- Une décision protocole non triviale → un commentaire citant la **source** (URL minecraft.wiki + version).

---

## 8. Git, commits & branches

- **Branches** : `feature/M<n>-<slug>` (ex. `feature/M1-spawn`), merge dans `master` quand la *Definition of Done* est verte.
- **Commits conventionnels** : `type(scope): sujet` — `feat`, `fix`, `test`, `docs`, `refactor`, `chore`, `perf`. Scope = jalon ou package (`feat(M1): decode Login Start`, `test(codec): VarInt golden vectors`).
- Commits atomiques (un paquet / une primitive / un fix par commit).
- Pas de secret, pas de binaire, pas d'artefact runtime commité (cf. `.gitignore`).

---

## 9. Checklist avant de pousser

```
[ ] gofmt -l .            → vide
[ ] go vet ./...          → OK
[ ] staticcheck ./...     → OK
[ ] go test -race ./...   → vert
[ ] go build ./cmd/server → OK
[ ] IDs PLAY touchés : vérifiés + commentaire source/version à jour
[ ] doc.go / PLAN / WORKFLOW à jour si le comportement a changé
[ ] aucun nouvel import non-stdlib
```

---

*Conventions établies le 2026-06-06. Voir [PLAN.md](PLAN.md) (architecture) et [WORKFLOW.md](WORKFLOW.md) (process).*
