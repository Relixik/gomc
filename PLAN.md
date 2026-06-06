# 📐 PLAN — Serveur Minecraft Java en Go (from scratch, stdlib-only)

> **Cible** : Minecraft Java Edition **26.1.2**, protocole réseau **775** (sortie 2026-04-09).
> **Principe** : **zéro dépendance tierce** — uniquement la bibliothèque standard Go. Objectif assumé : garder le contrôle total du protocole pour pouvoir le corriger / le mettre à jour à la main à chaque version.
> **Statut** : nouveau projet. L'ancien fork (protocole 578 / 1.15.2) est archivé dans [`old/`](old/) comme référence de lecture, hors du build.

Ce plan a été établi à partir d'une recherche multi-agents vérifiée contre [minecraft.wiki](https://minecraft.wiki/w/Protocol_version) (8 dimensions du protocole + vérification adversariale + synthèse).

---

## 1. Le piège n°1 à connaître avant tout

Le **protocole 775 a décalé TOUS les IDs de paquets de l'état PLAY** : ~15 nouveaux paquets *serverbound* ont été insérés, déplaçant les IDs suivants (ex. `custom_payload` sb `0x0A → 0x16`, `settings` `0x08 → 0x0E`, `keep_alive` `0x10 → 0x1C`, `chat_session_update` `0x06 → 0x0A`). Comptes corrigés : **69 serverbound / 141 clientbound** en PLAY.

- minecraft.wiki documente encore le **773 (1.21.10)**, pas le 775.
- Les tables `minecraft-data` pour 26.1.2 sont **connues comme buggées** (50 sb / 135 cb au lieu de 69/141).

👉 **Règle d'or** : les IDs des états *lifecycle* (Handshaking / Status / Login / Configuration) sont fiables depuis le wiki 773 et présumés stables en 775. Mais **chaque ID de l'état PLAY doit être vérifié octet par octet** contre un jar serveur 26.1.2 déobfusqué (ou un `minecraft-data` corrigé). C'est exactement là que 775 a cassé la compat. → voir [WORKFLOW.md](WORKFLOW.md#vérifier-les-ids-play).

---

## 2. Modèle mental du protocole

Une connexion = **un seul flux TCP** qui parcourt une machine à états :

```
Handshaking ──▶ Status   ──▶ (fin : ping serveur)
            └─▶ Login ──▶ Configuration ──▶ Play
                                  ▲              │
                                  └──────────────┘  (le serveur peut renvoyer en Configuration)
```

- Le **même octet d'ID** désigne des paquets différents selon `(état, direction)`. Le dispatch **doit** indexer sur `(State, Direction, ID)`, jamais sur l'ID seul.
- **Framing** : chaque paquet = `Length(VarInt) + PacketID(VarInt) + Data`. Après *Set Compression*, le cadre devient `PacketLength + DataLength(0 = non compressé) + zlib(ID+Data)`.
- **Chiffrement** : AES-128 **CFB8** (segment 8 bits), le secret partagé sert à la fois de clé ET d'IV, flux continu par direction, jamais réinitialisé. ⚠️ `crypto/cipher` (CFB 128 bits) est **faux** pour Minecraft.
- **Plus gros écart vs l'ancien code** : l'état **Configuration** (envoi des registres/tags) n'existait pas en 1.15.2 (les données de dimension étaient dans *Join Game*). Depuis 1.20.2 c'est une phase obligatoire — **aucun client moderne ne peut entrer en jeu sans elle**.

---

## 3. Architecture (packages Go)

Couches du bas vers le haut, chacune ne dépend que des couches inférieures :

| # | Package | Rôle | Stdlib utilisée |
|---|---------|------|-----------------|
| 1 | `internal/protocol/codec` | Types primitifs : VarInt/Long, String, Identifier, numériques BE, Angle, Position, UUID, BitSet, Optional/Array préfixés. | `encoding/binary`, `math`, `bufio`, `bytes` |
| 2 | `internal/protocol/nbt` | NBT réseau (racine **sans nom** depuis 1.20.2) + NBT disque (racine nommée, pour Anvil). Tags 0-12. | `encoding/binary`, `math` |
| 3 | `internal/protocol/text` | Text components (forme NBT principale + forme JSON legacy) ; struct Status. | `encoding/json`, `nbt` |
| 4 | `internal/protocol/frame` | Framing + pile de transformations `chiffrement(compression(cadre))`, double mode, caps anti-DoS, ping legacy `0xFE`. | `compress/zlib`, `crypto/aes`, `crypto/cipher`, `net`, `bufio` |
| 5 | `internal/protocol/auth` | RSA, clé publique DER, PKCS#1v1.5, hash SHA-1 signé Minecraft, hasJoined Mojang, UUIDv3 offline. | `crypto/{rsa,rand,x509,sha1,md5}`, `math/big`, `net/http`, `encoding/json` |
| 6 | `internal/protocol/packet` | Définitions de paquets + registre `(State,Dir,ID)→factory`. Un fichier par état/direction, IDs en consts commentées. | `codec`, `nbt`, `text` |
| 7 | `internal/protocol/registry` | Fournisseur de données de la phase Configuration : datapack vanilla embarqué → NBT réseau (Registry Data, Update Tags, Known Packs). | `embed`, `encoding/json`, `nbt` |
| 8 | `internal/net/session` | Machine à états par connexion + boucle de lecture + boucle d'écriture bornée. Handlers lifecycle inline ; handoff vers la game loop en Play. | `net`, `context`, `log/slog`, `time` |
| 9 | `internal/game/world` | Dimension, Chunk, paletted containers (blocs + biomes), heightmaps, sérialisation Chunk Data. | bit-packing pur Go |
| 10 | `internal/game/entity` | Modèle Entity/Player : position, rotation, allocation d'EID, metadata (plus tard). | pur Go |
| 11 | `internal/game/loop` | **Boucle de tick autoritaire 20 TPS** : intents entrants, registre des joueurs, keep-alive, broadcast, event bus, commandes. | `time`, `context`, `sync` |
| 12 | `internal/server` | Listener TCP, config (server.properties-like), shutdown gracieux, câblage. | `net`, `context`, `os/signal`, `bufio` |
| 13 | `cmd/server` | `main()` : flags, signaux, démarrage. | `flag`, `os`, `log/slog` |

### Modèle de concurrence (idiomatique Go)
- **1 goroutine de lecture + 1 goroutine d'écriture par connexion**. L'écriture draine un channel sortant **borné** ⇒ un socket lent ne bloque jamais la boucle de tick (overflow ⇒ drop + déconnexion).
- **1 seule goroutine "game loop" autoritaire** (ticker 50 ms + accumulateur de lag). **Toute** mutation du monde s'y fait ⇒ **pas de mutex** sur l'état du monde.
- Échanges inter-goroutines = **channels uniquement** : connexion → loop = un channel d'intents `(PlayerID, intent)` ; loop → connexion = enqueue sur le channel sortant du joueur.
- Cycle de vie de connexion possédé par un `context.Context` : à la fermeture, on annule, on draine/ferme le channel sortant, on poste **un** intent de retrait (la leçon du fix anti-fuite de goroutines de l'ancien code).
- **Codecs écrits à la main** (pas de réflexion sur tags de struct) : plus rapide, débogable, et rend les changements de champ par version évidents et localisés.

---

## 4. Le « zéro lib » en pratique

### À écrire à la main (pas d'équivalent stdlib — c'est le coût assumé du contrôle total)
- **VarInt/VarLong** : boucle 7 bits triviale, mais **cap 5/10 octets** + rejet overlong/overflow (anti-DoS). Two's-complement, pas zigzag.
- **CFB8** : le plus gros piège. `crypto/cipher.NewCFBEncrypter` = CFB **128 bits** = FAUX. Écrire un `cipher.Stream` 1 octet sur `crypto/aes` `block.Encrypt`. → portage direct de [`old/impl/conn/crypto/cfb8.go`](old/impl/conn/crypto/cfb8.go) (déjà fonctionnel et stdlib-only ✅).
- **NBT réseau** : racine sans nom depuis 1.20.2 ; le reader doit accepter une racine String (text components) ; modified-UTF-8 Java pour les chaînes NBT.
- **Text components** : encodage NBT (String / Compound) + forme JSON legacy (Login Disconnect = JSON ; Config/Play Disconnect = NBT).
- **Paletted containers** : bit-packing dans `[]int64` sans qu'une entrée chevauche un long ; single-valued / indirect (4-8 bpe) / direct ; heightmaps en long-arrays.
- **Hash SHA-1 signé Minecraft** : les 20 octets lus comme entier signé two's-complement, base-16, signe `-` optionnel (`math/big`).
- **UUIDv3 offline** : `MD5("OfflinePlayer:"+name)` + bits de version 3 / variante RFC-4122.
- **Registre de paquets + IDs par version** : tables maintenues à la main (volontaire — le décalage PLAY de 775 = une édition localisée).
- **Données de registres/datapack** : registres vanilla complets embarqués (`go:embed`) → NBT réseau.

### La stdlib suffit (ne PAS réinventer)
`compress/zlib` (compression) · `crypto/aes`,`crypto/rsa` (PKCS#1v1.5), `crypto/rand`, `crypto/x509` (`MarshalPKIXPublicKey` = exactement les octets DER de `getEncoded()` côté Java), `crypto/sha1`, `crypto/md5` · `net/http`+`encoding/json` (hasJoined Mojang) · `encoding/binary`+`math` (numériques BE) · `net`, `bufio`, `context`, `time`, `log/slog`, `embed`, `sync`.

> Suppression des anciennes deps : `google/uuid` → type 16 octets maison · `fatih/color` → escapes ANSI / slog · `durafmt` → `time`.

---

## 5. Roadmap par jalons

> Réaliste : la **parité vanilla complète n'est pas un livrable précoce**, c'est l'horizon (M7). Chaque jalon a une **méthode de vérification concrète**.

### ✅ M0 — Fondations : codec, NBT, frame, bascule du repo · *(1-1.5 sem)*
**Fait pour la partie squelette** : nouveau module à la racine, `cmd/server` ouvre un listener TCP, `old/` archivé. Reste à implémenter codec/nbt/text/frame/auth avec tests golden-bytes.
**Vérif** : `go test ./...` vert. Vecteurs golden : VarInt (`0,1,127,128,255,2147483647,-1⇒5 octets`), Position round-trip, NBT racine-sans-nom (+ racine String), CFB8 contre les vecteurs de référence de l'ancien code, hash serveur SHA-1 (`Notch`, `jeb_`, `simon`).

### 🎯 M1 — Se connecter, configurer, spawn et **BOUGER** dans un monde vide *(2-3 sem)* — **le jalon décisif**
**But** : un vrai client 26.1.2 non modifié fait Handshaking → Login (**offline**, sans chiffrement) → Configuration (registres minimaux mais complets) → Play, spawn dans un monde vide/void, et le joueur peut marcher/regarder.
**Paquets** :
- Handshaking sb `0x00` (Intent 1/2/3 ; peek `0xFE`)
- Status : sb `0x00` Request / cb `0x00` Response(JSON) / sb `0x01` Ping / cb `0x01` Pong
- Login : sb `0x00` Login Start / cb `0x02` Login Success (UUIDv3 offline) / sb `0x03` Login Acknowledged *(on saute Encryption/Set Compression en M1)*
- Configuration : sb `0x00` Client Information · Plugin Message `minecraft:brand` · cb `0x0C` Feature Flags · Known Packs (cb `0x0E`/sb `0x07`) · cb `0x07` Registry Data (dimension_type, biome dont `minecraft:plains`, 24 damage_type, chat_type, …) · cb `0x0D` Update Tags · cb `0x03` Finish Configuration / sb `0x03` Ack · Keep Alive
- Play *(IDs à VÉRIFIER vs jar 775)* : cb Login(play) · cb Game Event (event **13** = start waiting for level chunks) · cb Set Center Chunk · cb Chunk Data + Update Light (palette air single-valued, sections vides) · cb Synchronize Player Position ; sb Confirm Teleportation · sb Player Loaded · sb Set Player Position/Rotation · sb Keep Alive
**Vérif** : ① client réel 26.1.2 rejoint et bouge (capture courte) ② serveur visible dans la liste multijoueur (MOTD/online) ③ test d'intégration Go scriptant la séquence d'octets exacte ④ diff de capture vs serveur vanilla 26.1.2. Keep-Alive maintient la connexion > 60 s.

### M2 — Login online-mode (chiffrement) + compression *(1 sem)*
**But** : auth Mojang (AES/CFB8) + zlib ⇒ comptes premium + framing identique à vanilla.
**Paquets** : cb `0x01` Encryption Request (+ bool *Should Authenticate* 1.20.5+) · sb `0x01` Encryption Response · cb `0x03` Set Compression.
**Vérif** : compte premium rejoint ; round-trip `encrypt(compress(frame))` ; tests PKCS#1v1.5 + clé DER ; diff de capture à la frontière Set Compression / Login Success.

### M3 — Vrai monde plat + streaming de chunks + view-distance *(2 sem)*
**But** : superflat correct (bedrock/dirt/grass), paletted containers + heightmaps justes, (dé)chargement dynamique selon la distance de vue.
**Paquets** : cb Chunk Data + Update Light (palettes indirect/direct, biomes) · cb Set Center Chunk · cb Unload Chunk · cb Update Light.
**Vérif** : sol solide visible, aucun chunk noir en marchant loin ; tests d'encodage paletted (single/indirect/direct) + heightmaps ; diff d'un chunk superflat vanilla.

### M4 — Présence multijoueur : autres joueurs, chat, interaction de base *(2-3 sem)*
**Paquets** : cb Spawn Entity/Player · Player Info Update (add player) · Set Entity Position/Rotation (+deltas) · Set Head Rotation · Remove Entities · Player Chat / System Chat (770+ : repr SNBT/NBT) · sb Chat Message/Command · Block Update · sb Player Action (dig) / Use Item On (place).
**Vérif** : deux clients réels se voient bouger + chat global + pose/casse de bloc partagés ; test de fan-out broadcast.

### M5 — Inventaire, items, metadata d'entités, temps/météo *(3-4 sem)*
**Paquets** : Set Container Content/Slot · sb Click Container / Set Held Item · Set Entity Metadata · Spawn Entity (items) · Update Time · Game Event (pluie) · Set Equipment.

### M6 — Persistance (Anvil .mca) + commandes + maturité des events *(4-6 sem)*
**But** : sauvegarde/chargement monde (régions Anvil, level.dat) + données joueur ; systèmes de commandes/events matures.
**Paquets** : cb Commands (graphe Brigadier) · sb Command Suggestions Request · cb Suggestions Response (+ sérialisation NBT-disque, pas un paquet).
**Vérif** : le monde survit au redémarrage ; round-trip région Anvil ; graphe Brigadier envoyé pour l'auto-complétion.

### M7 — Horizon parité vanilla (longue traîne) · *open-ended*
Mobs/IA, redstone, crafting, worldgen au-delà du superflat, combat, dimensions/portails. Les ~60 sb / ~135 cb paquets PLAY restants, ajoutés incrémentalement, **chacun vérifié octet par octet vs le jar 775**. Jamais « fini », toujours plus proche.

---

## 6. Risques & mitigations

| # | Risque | Mitigation |
|---|--------|-----------|
| 1 | **IDs PLAY 775 non documentés** (wiki=773, minecraft-data buggé). Deviner ⇒ mauvais décodage silencieux. | Vérifier chaque ID PLAY contre un jar 26.1.2 déobfusqué ; IDs en consts commentées par source/version ; test de conformité loggant le 1er octet de chaque paquet sb. |
| 2 | **CFB8 vs CFB128 stdlib** : désync après le 1er octet. | Porter + tester le CFB8 8 bits de l'ancien code ; vecteurs connus ; online-mode derrière M2 (M1 = offline). |
| 3 | **Racine NBT nommée par erreur** ⇒ rejet de toute la phase Configuration. | NBT réseau sans-nom par défaut ; NBT disque (nommé) en mode séparé (M6) ; round-trip golden + diff Registry Data vanilla. |
| 4 | **Registres insuffisants** : client rejette Login(play) sans 24 damage_type, ≥1 dimension_type, biome `minecraft:plains` (l'ordre = IDs numériques). | Embarquer le datapack 26.1.2 complet, émettre verbatim en ordre déterministe ; vérifier 24 damage_type ; Known Packs correct (le serveur **bloque** jusqu'aux Known Packs client). |
| 5 | **Configuration = plus gros écart vs old/** : réutiliser les vieux modèles ⇒ aucun client ne se connecte. | Traiter Configuration comme état de 1ère classe dès le jour 1 ; acceptation M1 = vrai client passe config→play. |
| 6 | **Frontière framing/chiffrement off-by-one** : Set Compression doit précéder Login Success ; chiffrement actif juste après Encryption Response. | Centraliser le switch dans FrameConn aux frontières d'octets exactes + tests des paquets-frontière ; diff de capture en M2. |
| 7 | Configuration lente (gros NBT) ou écriture bloquée ⇒ blocage tick / timeout Keep-Alive. | Channel sortant borné + goroutine d'écriture dédiée ; Keep-Alive en Configuration ET Play ; drop+disconnect sur overflow. |
| 8 | **Fuites de goroutines** à la déconnexion (le bug que T002.3 avait corrigé). | `context.Context` par connexion ; à la fermeture : cancel + drain/close sortant + 1 intent de retrait ; test de fuite (compter les goroutines sur N cycles). |
| 9 | **Anti-DoS** : VarInt de longueur malveillant / paquet géant / ping `0xFE`. | Caps 3 octets (length) / 2097151 octets (paquet) / 5-10 (VarInt) + détection peek `0xFE`, tout dans la couche frame avant toute allocation. |
| 10 | **Version churn** : 775 sera dépassé, tables manuelles à risque de rot. | 1 fichier par état/direction + consts d'ID citées + tests golden ⇒ bump de version = édition localisée test-guardée (le but du design sans codegen). |

---

## 7. Stratégie repo

- **`old/`** = ancien module (`github.com/Relixik/minecraft-server`, son propre `go.mod`) ⇒ exclu automatiquement du build du nouveau module (`go build ./...` ne descend pas dans un sous-module). Conservé comme référence (notamment `cfb8.go`).
- **Nouveau module** à la racine : `github.com/Relixik/gomc` (Go 1.26, **aucun `require`**, `go.sum` vide). Renommable : `go mod edit -module <chemin>`.
- **Arbo** : `cmd/server` · `internal/protocol/{codec,nbt,text,frame,auth,packet,registry}` · `internal/net/session` · `internal/game/{world,entity,loop}` · `internal/server` · `testdata/` (golden + captures vanilla) · `data/` (datapack embarqué).
- **Nouveau repo GitHub** : y pousser uniquement les nouveaux fichiers (pas `old/`). Avant d'exclure `old/`, taguer l'état actuel (`pre-rewrite`) pour préserver l'archive dans l'historique. → détails dans [WORKFLOW.md](WORKFLOW.md#bascule-vers-le-nouveau-repo).
- **CI** (GitHub Actions, trivial car zéro dep) : `go vet`, `gofmt -l`, `go test -race`, `go build ./cmd/server`, + `staticcheck` / `govulncheck`. Job séparé (nightly/manuel) pour le diff de capture vs serveur 26.1.2.

---

*Plan généré le 2026-06-06 à partir d'une recherche protocole vérifiée. Sources : [minecraft.wiki/Protocol_version](https://minecraft.wiki/w/Protocol_version), [Java Edition 26.1.2](https://minecraft.wiki/w/Java_Edition_26.1.2), PrismarineJS/mineflayer #3888.*
