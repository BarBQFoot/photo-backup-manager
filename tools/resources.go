package aitools

import "embed"

// Worker and setup scripts are bundled into the app; first launch prepares Python and weights.
//
//go:embed app_caption_worker.py caption_images.py translate_captions.py validate_captions.py prepare_translation.py bootstrap_ai.py
var Files embed.FS
