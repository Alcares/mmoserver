// crewmate.js - Standalone Among Us Character Renderer
(function (global) {
    // Preset Palette Definitions
    const PALETTES = {
        red:    { main: '#C51111', shadow: '#7A0838', visor: '#38BDF8', visorShadow: '#0284C7' },
        blue:   { main: '#132ED1', shadow: '#09158E', visor: '#38BDF8', visorShadow: '#0284C7' },
        green:  { main: '#117F2D', shadow: '#0A4D1A', visor: '#38BDF8', visorShadow: '#0284C7' },
        pink:   { main: '#ED54BA', shadow: '#AB2BA0', visor: '#38BDF8', visorShadow: '#0284C7' },
        orange: { main: '#EF7D0D', shadow: '#B04A00', visor: '#38BDF8', visorShadow: '#0284C7' },
        yellow: { main: '#F5F557', shadow: '#C29C1C', visor: '#38BDF8', visorShadow: '#0284C7' },
        black:  { main: '#3F474E', shadow: '#1E2124', visor: '#38BDF8', visorShadow: '#0284C7' },
        white:  { main: '#D6E0F0', shadow: '#8394BF', visor: '#38BDF8', visorShadow: '#0284C7' },
        purple: { main: '#6B2FBB', shadow: '#3B177C', visor: '#38BDF8', visorShadow: '#0284C7' },
        cyan:   { main: '#38E2DD', shadow: '#11939A', visor: '#38BDF8', visorShadow: '#0284C7' }
    };

    const PALETTE_KEYS = Object.keys(PALETTES);

    class CrewmateRenderer {
        /**
         * Gets color set based on ID or index
         */
        static getColor(colorKeyOrId) {
            if (typeof colorKeyOrId === 'number') {
                const key = PALETTE_KEYS[Math.abs(colorKeyOrId) % PALETTE_KEYS.length];
                return PALETTES[key];
            }
            return PALETTES[colorKeyOrId] || PALETTES.red;
        }

        /**
         * Draw an animated Crewmate centered at (x, y)
         * @param {CanvasRenderingContext2D} ctx
         * @param {Object} options Configuration options
         */
        static draw(ctx, options = {}) {
            const {
                x = 0,
                y = 0,
                size = 22,             // Target height/width bounding size
                vx = 0,               // Horizontal velocity (-1 to 1)
                vy = 0,               // Vertical velocity (-1 to 1)
                facingLeft = false,
                color = 'red',
                isGhost = false,
                time = performance.now()
            } = options;

            const palette = typeof color === 'string' ? (PALETTES[color] || PALETTES.red) : color;
            const isMoving = Math.hypot(vx, vy) > 0.1;

            // Animation parameters
            const walkCycle = isMoving ? (time * 0.012) % (Math.PI * 2) : 0;
            const idleBob = !isMoving ? Math.sin(time * 0.004) * 1.5 : 0;
            const legOffset = isMoving ? Math.sin(walkCycle) * 4 : 0;
            const bodyTilt = isMoving ? Math.sin(walkCycle * 0.5) * 0.08 : 0;

            ctx.save();
            ctx.translate(x, y + idleBob);

            // Horizontal flip handling
            if (facingLeft) {
                ctx.scale(-1, 1);
            }

            // Scale to match target tile dimensions
            const scale = size / 28;
            ctx.scale(scale, scale);

            if (isGhost) {
                ctx.globalAlpha = 0.6;
            }

            ctx.lineWidth = 2.5;
            ctx.lineJoin = 'round';
            ctx.lineCap = 'round';

            // 1. Backpack
            ctx.beginPath();
            ctx.roundRect(-14, -6, 7, 14, 3);
            ctx.fillStyle = palette.shadow;
            ctx.fill();
            ctx.strokeStyle = '#000000';
            ctx.stroke();

            // 2. Legs (if not ghost)
            if (!isGhost) {
                // Back Leg
                ctx.beginPath();
                ctx.roundRect(-6 + legOffset, 6, 6, 10, 3);
                ctx.fillStyle = palette.shadow;
                ctx.fill();
                ctx.stroke();

                // Front Leg
                ctx.beginPath();
                ctx.roundRect(1 - legOffset, 6, 6, 10, 3);
                ctx.fillStyle = palette.main;
                ctx.fill();
                ctx.stroke();
            }

            // 3. Main Body
            ctx.save();
            ctx.rotate(bodyTilt);

            ctx.beginPath();
            if (isGhost) {
                // Ghost tail curve
                ctx.moveTo(-9, -12);
                ctx.bezierCurveTo(-11, -2, -10, 8, -6, 12);
                ctx.bezierCurveTo(-2, 15, 2, 10, 6, 14);
                ctx.bezierCurveTo(9, 10, 10, -2, 9, -12);
                ctx.arc(0, -12, 9, Math.PI, 0, false);
            } else {
                // Standard capsule body
                ctx.roundRect(-9, -16, 18, 24, 8);
            }
            ctx.fillStyle = palette.main;
            ctx.fill();
            ctx.strokeStyle = '#000000';
            ctx.stroke();

            // Shadow overlay on lower body
            ctx.beginPath();
            ctx.roundRect(-8, 0, 16, 7, [0, 0, 6, 6]);
            ctx.fillStyle = palette.shadow;
            ctx.fill();

            // 4. Visor
            ctx.beginPath();
            ctx.ellipse(3, -7, 6.5, 4.5, 0, 0, Math.PI * 2);
            ctx.fillStyle = palette.visor;
            ctx.fill();
            ctx.strokeStyle = '#000000';
            ctx.stroke();

            // Visor Highlight
            ctx.beginPath();
            ctx.ellipse(4.5, -8.5, 2.5, 1.2, -Math.PI / 6, 0, Math.PI * 2);
            ctx.fillStyle = '#FFFFFF';
            ctx.fill();

            ctx.restore();
            ctx.restore();
        }
    }

    global.CrewmateRenderer = CrewmateRenderer;
})(window);