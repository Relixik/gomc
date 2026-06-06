# 🔧 WORKFLOW — Comment on construit gomc

Process de développement pour le serveur défini dans [PLAN.md](PLAN.md). Court, opérationnel, et adapté au fait qu'on cible un protocole (775) **mal documenté publiquement** et qu'on s'interdit toute lib.

---

## 1. Boucle de développement (par jalon)

On avance **un jalon à la fois** (M0 → M7), jamais en parallèle. Chaque jalon suit la même boucle :

```
1. SCOPE   → relire la section du jalon dans PLAN.md ; lister les paquets concernés.
2. SPEC    → pour chaque paquet : récupérer le format exact (champs, types, ordre) depuis
             minecraft.wiki + VÉRIFIER l'ID (voir §3 si état PLAY).
3. CODE    → implémenter le paquet (struct + Encode/Decode) dans le bon fichier
             internal/protocol/packet/<state>_<dir>.go, ID en const commentée.
4. TEST    → test golden-bytes (table-driven) qui fige le format sur le fil.
5. WIRE    → brancher le handler (session pour lifecycle, intent→loop pour Play).
6. VERIFY  → exécuter le harnais de vérification (§2) au niveau du jalon.
7. COMMIT  → sur la branche feature/M<n>-<slug>, message conventionnel.
```

Une seule chose à la fois sur le fil : **on ne passe pas au paquet/jalon suivant tant que le harnais n'est pas vert.**

---

## 2. Le harnais de vérification protocole (3 niveaux)

C'est le cœur du projet : sans lib, la seule garantie de conformité vient de la **vérification empirique**.

### Niveau 1 — Tests golden-bytes (unitaire, rapide, en CI)
Chaque paquet a une table `{valeur Go ⇄ octets attendus}`. Encode produit exactement ces octets ; Decode les relit à l'identique. Pour les primitives : vecteurs canoniques (VarInt `0,1,127,128,255,2147483647,-1`, Position round-trip, CFB8 vs vecteurs de l'ancien code, hash SHA-1 `Notch`/`jeb_`/`simon`).

### Niveau 2 — Test d'intégration scripté (Go, en CI)
Un faux client en Go (toujours stdlib) qui ouvre une connexion TCP locale, joue la **séquence d'octets exacte** d'un client réel pour le jalon, et **assert chaque paquet clientbound** reçu (ID + champs). C'est le test qui prouve « un client passe handshake→login→config→play » sans lancer Minecraft.

### Niveau 3 — Diff de capture vs serveur vanilla 26.1.2 (manuel / nightly)
La source de vérité ultime. On lance **deux** cibles avec le **même client** faisant les **mêmes actions**, et on compare les flux paquet par paquet :

```
                 ┌─────────────┐         capture A
   client 26.1.2 │  proxy MITM │ ───────────────────▶ serveur VANILLA 26.1.2 (officiel)
   (le même)     │  qui logue  │
                 └─────────────┘         capture B
                        └────────────────────────────▶ notre serveur gomc
   diff(A, B)  → les IDs/ordre/format doivent coïncider (le contenu des registres peut différer)
```

- **Proxy MITM** : un petit relais TCP (stdlib) qui décode le framing (et le chiffrement une fois M2 fait) et logue `(état, direction, id, len)` de chaque paquet. À écrire en M1, réutilisé partout ensuite.
- ⚠️ Un serveur **Paper/Spigot/plugins n'est PAS un oracle de protocole** : les plugins vivent au-dessus des paquets. La référence, c'est le **serveur vanilla officiel** + capture.

> Règle : **niveaux 1 & 2 obligatoires en CI** ; **niveau 3 obligatoire pour clôturer un jalon** (au moins une fois, capture archivée dans `testdata/`).

---

## 3. Vérifier les IDs PLAY

Comme expliqué dans [PLAN.md §1](PLAN.md#1-le-piège-n1-à-connaître-avant-tout), les IDs PLAY de 775 ne sont pas fiables publiquement. Procédure :

1. **Source primaire** : un **jar serveur 26.1.2 déobfusqué** (mappings officiels Mojang) — la table `(état, direction, id) → paquet` y est extractible. C'est la vérité.
2. **Source de secours** : `minecraft-data` **après** correction de l'issue PrismarineJS #3888 (à surveiller).
3. **Validation empirique** : le proxy MITM (niveau 3) logue le **1er octet de chaque paquet serverbound** quand on déclenche une action connue côté client → on confirme l'ID réel.
4. **Documentation** : chaque ID est une `const` avec un commentaire citant sa source/version :
   ```go
   const idKeepAlive = 0x1C // serverbound Play — vérifié vs jar 26.1.2 (2026-06-xx)
   ```
   Tant qu'un ID PLAY n'est que « présumé 773 », il porte le commentaire `// 773 wiki, à confirmer 775` et n'est pas considéré comme acquis.

---

## 4. Definition of Done (par jalon)

- [ ] Tous les paquets du jalon implémentés (Encode/Decode) avec IDs vérifiés et cités.
- [ ] Tests golden-bytes verts pour chaque paquet (niveau 1).
- [ ] Test d'intégration scripté vert (niveau 2).
- [ ] Diff de capture vs vanilla passé au moins une fois, capture archivée dans `testdata/` (niveau 3).
- [ ] `go vet ./...`, `gofmt -l .` (vide), `go test -race ./...` verts.
- [ ] Critère d'acceptation **fonctionnel** du jalon validé (ex. M1 : vrai client 26.1.2 spawn et bouge sans kick > 60 s).
- [ ] Pas de fuite de goroutine (test de cycle connect/disconnect).

---

## 5. Git & CI

- **Branches** : `feature/M<n>-<slug>` (ex. `feature/M1-spawn`), merge dans `master` quand la DoD est verte. (On continue le pattern feature-branch + merge déjà présent dans l'historique.)
- **Commits** : conventionnels (`feat(M1): decode Handshake packet`, `test(codec): VarInt golden vectors`).
- **CI GitHub Actions** (rapide, zéro dep à mettre en cache) :
  ```yaml
  jobs:
    check:
      strategy: { matrix: { go: ['1.26'] } }
      steps:
        - go vet ./...
        - test -z "$(gofmt -l .)"
        - go test -race ./...
        - go build ./cmd/server
        - govulncheck ./...   # surface faible (stdlib only) mais bonne hygiène
  ```
- **Job séparé** (nightly/manuel, derrière un flag) : diff de capture vs un serveur 26.1.2 conteneurisé.
- **Protection de branche** : CI verte requise pour merger.

---

## 6. Bascule vers le nouveau repo

Quand tu voudras publier le nouveau repo GitHub (qui ne sera plus un fork) :

```bash
# 1. Préserver l'archive de l'ancien code dans l'historique (avant de l'exclure)
git add -A && git commit -m "chore: archive old 1.15.2 fork into old/, bootstrap gomc skeleton"
git tag pre-rewrite        # l'ancien code reste atteignable via ce tag

# 2. Option A — réutiliser ce repo : exclure old/ du suivi
git rm -r --cached old/    # garde les fichiers sur disque, les retire de l'index
# (old/ est déjà dans .gitignore) puis commit

# 2. Option B — repo vierge : copier uniquement les nouveaux fichiers
#    cmd/ internal/ go.mod PLAN.md WORKFLOW.md README.md LICENSE  (PAS old/)
#    dans un nouveau dossier, git init, premier commit propre.
```

> `old/` est déjà dans `.gitignore`, mais comme il est actuellement **suivi** dans ce repo, il faut un `git rm --cached` explicite pour l'en sortir (le `.gitignore` seul ne dé-suit pas un fichier déjà commité).

---

## 7. Où Claude intervient

À chaque jalon je peux prendre en charge, de façon vérifiée :
- **Récupérer + figer la spec** d'un paquet (format exact depuis minecraft.wiki) et écrire son Encode/Decode + test golden.
- **Écrire le proxy MITM** de capture et le test d'intégration scripté.
- **Implémenter** un package complet (codec, nbt, frame…) avec ses tests.
- **Diffuser une recherche** quand un point protocole est ambigu (workflow multi-agents + vérif adversariale, comme pour ce plan).

Prochaine étape proposée : **finir M0** (implémenter `codec` + `nbt` + `frame` + `auth` avec leurs tests golden), puis attaquer **M1**.

---

*Workflow établi le 2026-06-06. Voir [PLAN.md](PLAN.md) pour l'architecture et la roadmap détaillées.*
