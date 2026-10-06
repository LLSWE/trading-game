--- GEN WALLET WITHOUT CALLING 

INSERT INTO wallets (id, player_id, currency, balance, version)
VALUES (
    '0192f291-27dd-7d3f-8071-5f8685deef37',
    'player-test-1',
    'BRL',
    10000, -- R$ 100.00
    1
) ON CONFLICT (id) DO NOTHING;

