# gomc — Serveur Minecraft Java en Go (from scratch, stdlib-only)

> Implémentation propre d'un serveur **Minecraft Java Edition 26.1.2** (protocole **775**) écrite en Go, **sans aucune dépendance tierce** — uniquement la bibliothèque standard.

## Pourquoi « zéro lib » ?

Pour garder le **contrôle total du protocole** : pouvoir corriger un paquet ou suivre une nouvelle version de Minecraft à la main, sans attendre qu'une bibliothèque externe se mette à jour. Tout ce qui touche au fil réseau (VarInt, NBT, CFB8, text components, paletted chunks…) est écrit ici ; la stdlib fournit le reste (crypto, zlib, http, json, binary).

> ⚠️ Ce projet n'est **pas** un fork : il repart de zéro. L'ancien serveur (1.15.2 / protocole 578) est archivé dans [`old/`](old/) comme simple référence.

## État

🚧 **M0 — Fondations** (en cours). Le serveur ouvre un listener TCP ; les couches protocole sont en cours d'implémentation. Voir la roadmap dans [PLAN.md](PLAN.md).

## Démarrer

```bash
go run ./cmd/server                 # écoute sur 0.0.0.0:25565
go run ./cmd/server -port 25599 -debug
go test ./...                       # tests golden-bytes
```

Prérequis : **Go 1.26+**. Aucune dépendance à télécharger (`go.sum` est vide, c'est voulu).

## Documentation

| Fichier | Contenu |
|---------|---------|
| [PLAN.md](PLAN.md) | Architecture, les 13 packages, le « zéro lib » en pratique, roadmap M0→M7, risques. |
| [WORKFLOW.md](WORKFLOW.md) | Process de dev, harnais de vérification protocole (3 niveaux), vérif des IDs PLAY, Git/CI. |
| [CONVENTIONS.md](CONVENTIONS.md) | Conventions de code, règles de maintenabilité, standards de test et de documentation. |

## Architecture en un coup d'œil

```
cmd/server                  → main()
internal/
  protocol/
    codec      → types primitifs (VarInt, String, Position…)
    nbt        → NBT réseau (racine sans nom) + disque
    text       → text components (NBT + JSON legacy)
    frame      → framing + chiffrement(compression(cadre)), CFB8
    auth       → RSA, hash serveur, hasJoined Mojang, UUID offline
    packet     → définitions + registre (State,Dir,ID)→factory
    registry   → données de la phase Configuration (datapack embarqué)
  net/session  → machine à états par connexion
  game/
    world      → chunks, paletted containers, heightmaps
    entity     → joueurs / entités
    loop       → boucle de tick autoritaire 20 TPS
  server       → listener TCP, config, shutdown
```

Modèle : 1 goroutine lecture + 1 écriture par connexion, **une seule** goroutine autoritaire pour le monde (pas de mutex), échanges par channels. Détails dans [PLAN.md](PLAN.md#3-architecture-packages-go).

## Licence

Voir [LICENSE](LICENSE).
