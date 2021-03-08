# Chess plugin

This plugin integrates [chess](https://github.com/notnil/chess) library in Mattermost, so you can play chess with other users in Mattermost.

## Getting Started

To challenge any user, just write `/chess challenge @someone`. This will start a game in the direct message channel with that user.

To move your piece, you have to click the move button, and add the [Standard Algebraic Notation](https://en.wikipedia.org/wiki/Algebraic_notation_(chess)) of the move you want to make. Examples:
- e6 = Move the Pawn on column e to row 6.
- Nc3 = Move the Knight to column c, row 3.
- Raa6 = Move the Rook on colum a to column a, row 6
- xe5 = Capture with the Pawn the piece at column e, row 5
- N6xf4 = Capture with the Knight on row 6 the piece on column f, row 4
- e8Q = Move the pawn to column e, row 8, and promote to Queen
- 0-0 = King side castling
- 0-0-0 = Queen side castling

Each user can only have one active game per user.

You can resign a game by hitting the "Resign" button.
