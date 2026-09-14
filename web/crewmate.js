// crewmate.js - Animated Suited Trader with Human Face (Among Us Silhouette)
(function (global) {
    const PALETTES = {
        // Fair / Peach
        navy:      { suit: '#1e3a8a', suitShadow: '#172554', tie: '#dc2626', skin: '#ffd8c4', hair: '#3a2010' },
        // Warm Beige / Tan
        charcoal:  { suit: '#334155', suitShadow: '#1e293b', tie: '#0284c7', skin: '#f3c5a5', hair: '#1c1917' },
        // Olive / Medium Tan
        black:     { suit: '#18181b', suitShadow: '#09090b', tie: '#eab308', skin: '#dfa67b', hair: '#451a03' },
        // Rich Warm Brown
        emerald:   { suit: '#065f46', suitShadow: '#064e3b', tie: '#f59e0b', skin: '#a5694f', hair: '#171717' },
        // Deep Espresso Brown
        oxblood:   { suit: '#881337', suitShadow: '#4c0519', tie: '#38bdf8', skin: '#6e432d', hair: '#0a0a0a' },
        // Neutral Light
        champagne: { suit: '#94a3b8', suitShadow: '#64748b', tie: '#2563eb', skin: '#fcd5be', hair: '#292524' }
    };

    const PALETTE_KEYS = Object.keys(PALETTES);

    class CrewmateRenderer {
        static getColor(colorKeyOrId) {
            if (typeof colorKeyOrId === 'number') {
                const key = PALETTE_KEYS[Math.abs(colorKeyOrId) % PALETTE_KEYS.length];
                return PALETTES[key];
            }
            return PALETTES[colorKeyOrId] || PALETTES.navy;
        }

        static draw(ctx, options = {}) {
            const {
                x = 0,
                y = 0,
                size = 32,
                vx = 0,
                vy = 0,
                facingLeft = false,
                color = 'navy',
                isGhost = false,
                time = performance.now()
            } = options;

            const pal = typeof color === 'string' ? (PALETTES[color] || PALETTES.navy) : color;

            const speed = Math.hypot(vx, vy);
            const isMoving = speed > 0.01;

            // Movement & walk cycle
            const walkCycle = isMoving ? (time * 0.02) : 0;
            const bob = isMoving ? Math.abs(Math.sin(walkCycle)) * -2.5 : Math.sin(time * 0.0035) * 1.2;
            const lean = isMoving ? 0.12 : 0;

            const leg1 = isMoving ? Math.sin(walkCycle) * 6 : 0;
            const leg2 = isMoving ? -Math.sin(walkCycle) * 6 : 0;
            const armSwing = isMoving ? Math.sin(walkCycle) * 5 : 0;

            ctx.save();
            ctx.translate(x, y + bob);

            if (facingLeft) {
                ctx.scale(-1, 1);
            }

            // Normalization scale
            const scale = size / 32;
            ctx.scale(scale, scale);

            if (isGhost) {
                ctx.globalAlpha = 0.5;
            }

            ctx.lineWidth = 2.2;
            ctx.lineCap = 'round';
            ctx.lineJoin = 'round';

            // --- 1. BACK ARM & BRIEFCASE ---
            ctx.save();
            ctx.translate(-7 - armSwing * 0.5, 3);

            // Back hand
            ctx.beginPath();
            ctx.arc(0, 0, 3, 0, Math.PI * 2);
            ctx.fillStyle = pal.skin;
            ctx.fill();
            ctx.strokeStyle = '#000';
            ctx.stroke();

            // Leather Briefcase
            ctx.save();
            ctx.translate(-1, 2);
            ctx.rotate(isMoving ? Math.sin(walkCycle) * 0.2 : 0);
            ctx.beginPath();
            ctx.roundRect(-2, 0, 11, 8, 2);
            ctx.fillStyle = '#78350f';
            ctx.fill();
            ctx.strokeStyle = '#000';
            ctx.stroke();
            // Brass lock
            ctx.fillStyle = '#facc15';
            ctx.fillRect(2.5, 3, 2, 2);
            // Handle
            ctx.beginPath();
            ctx.arc(3.5, 0, 2, Math.PI, 0, false);
            ctx.strokeStyle = '#292524';
            ctx.lineWidth = 1.2;
            ctx.stroke();
            ctx.restore();

            ctx.restore();

            // --- 2. BACK LEG ---
            if (!isGhost) {
                ctx.beginPath();
                ctx.roundRect(-6 + leg2, 6, 6, 9, 2);
                ctx.fillStyle = pal.suitShadow;
                ctx.fill();
                ctx.strokeStyle = '#000';
                ctx.stroke();

                // Black Oxford Shoe
                ctx.beginPath();
                ctx.roundRect(-7 + leg2, 13, 7.5, 3.5, [2, 3, 1, 1]);
                ctx.fillStyle = '#0f172a';
                ctx.fill();
                ctx.strokeStyle = '#000';
                ctx.stroke();
            }

            // --- 3. BODY & HEAD TORSO ---
            ctx.save();
            ctx.rotate(lean);

            // Suited Body (lower bean)
            ctx.beginPath();
            ctx.roundRect(-8, -5, 16, 15, [0, 0, 6, 6]);
            ctx.fillStyle = pal.suit;
            ctx.fill();
            ctx.strokeStyle = '#000';
            ctx.stroke();

            // Suit side shadow
            ctx.beginPath();
            ctx.roundRect(-8, -2, 5, 12, [0, 0, 0, 6]);
            ctx.fillStyle = pal.suitShadow;
            ctx.fill();

            // White Shirt V-collar
            ctx.beginPath();
            ctx.moveTo(-2, -5);
            ctx.lineTo(6, -5);
            ctx.lineTo(2, 3);
            ctx.closePath();
            ctx.fillStyle = '#ffffff';
            ctx.fill();

            // Red Silk Tie
            ctx.beginPath();
            ctx.moveTo(1, -4);
            ctx.lineTo(3.5, -4);
            ctx.lineTo(4.5, 4);
            ctx.lineTo(2.2, 7);
            ctx.lineTo(0.5, 4);
            ctx.closePath();
            ctx.fillStyle = pal.tie;
            ctx.fill();
            ctx.strokeStyle = '#991b1b';
            ctx.lineWidth = 0.8;
            ctx.stroke();

            // --- 4. HUMAN HEAD & FACE ---
            // Head base (Skin tone)
            ctx.lineWidth = 2.2;
            ctx.beginPath();
            ctx.arc(0, -11, 8.5, 0, Math.PI * 2);
            ctx.fillStyle = pal.skin;
            ctx.fill();
            ctx.strokeStyle = '#000';
            ctx.stroke();

            // Slicked-back Hair (top & back)
            ctx.beginPath();
            ctx.moveTo(-8.5, -11);
            ctx.quadraticCurveTo(-9, -20, 2, -19.5);
            ctx.quadraticCurveTo(8, -19, 7.5, -13);
            ctx.quadraticCurveTo(3, -15, -4, -13);
            ctx.quadraticCurveTo(-7, -11, -8.5, -11);
            ctx.fillStyle = pal.hair;
            ctx.fill();
            ctx.strokeStyle = '#000';
            ctx.stroke();

            // Trader Ear
            ctx.beginPath();
            ctx.arc(-4, -10, 2.3, 0, Math.PI * 2);
            ctx.fillStyle = pal.skin;
            ctx.fill();
            ctx.strokeStyle = '#000';
            ctx.lineWidth = 1.4;
            ctx.stroke();

            // Nose
            ctx.beginPath();
            ctx.moveTo(7.5, -12);
            ctx.lineTo(9.5, -10);
            ctx.lineTo(7, -9);
            ctx.strokeStyle = '#000';
            ctx.lineWidth = 1.8;
            ctx.stroke();

            // Big Stylized Cartoon Eye
            ctx.beginPath();
            ctx.ellipse(4, -11.5, 3.2, 4, 0, 0, Math.PI * 2);
            ctx.fillStyle = '#ffffff';
            ctx.fill();
            ctx.strokeStyle = '#000';
            ctx.lineWidth = 1.6;
            ctx.stroke();

            // Pupil looking forward
            ctx.beginPath();
            ctx.arc(5.2, -11.5, 1.6, 0, Math.PI * 2);
            ctx.fillStyle = '#0f172a';
            ctx.fill();

            // Catchlight (twinkle)
            ctx.beginPath();
            ctx.arc(5.8, -12.3, 0.6, 0, Math.PI * 2);
            ctx.fillStyle = '#ffffff';
            ctx.fill();

            // Determined Eyebrow
            ctx.beginPath();
            ctx.moveTo(1.5, -16.5);
            ctx.lineTo(7, -15);
            ctx.strokeStyle = pal.hair;
            ctx.lineWidth = 2;
            ctx.stroke();

            // Smirk / Determined Mouth
            ctx.beginPath();
            ctx.moveTo(3, -6.5);
            ctx.quadraticCurveTo(6, -6.5, 7, -8);
            ctx.strokeStyle = '#000';
            ctx.lineWidth = 1.5;
            ctx.stroke();

            ctx.restore(); // end body tilt

            // --- 5. FRONT LEG ---
            if (!isGhost) {
                ctx.lineWidth = 2.2;
                ctx.beginPath();
                ctx.roundRect(1 + leg1, 6, 6, 9, 2);
                ctx.fillStyle = pal.suit;
                ctx.fill();
                ctx.strokeStyle = '#000';
                ctx.stroke();

                // Front Shoe
                ctx.beginPath();
                ctx.roundRect(0 + leg1, 13, 8.5, 3.5, [2, 3, 1, 1]);
                ctx.fillStyle = '#0f172a';
                ctx.fill();
                ctx.strokeStyle = '#000';
                ctx.stroke();
            }

            // --- 6. FRONT ARM & HAND ---
            ctx.save();
            ctx.translate(3 + armSwing, 3);

            // Suit sleeve
            ctx.beginPath();
            ctx.arc(0, 0, 3.8, 0, Math.PI * 2);
            ctx.fillStyle = pal.suit;
            ctx.fill();
            ctx.strokeStyle = '#000';
            ctx.lineWidth = 2;
            ctx.stroke();

            // White shirt cuff
            ctx.fillStyle = '#ffffff';
            ctx.fillRect(1, -2, 1.5, 4);

            // Front Hand (clenched fist stride)
            ctx.beginPath();
            ctx.arc(3.2, 0, 2.7, 0, Math.PI * 2);
            ctx.fillStyle = pal.skin;
            ctx.fill();
            ctx.strokeStyle = '#000';
            ctx.lineWidth = 1.6;
            ctx.stroke();

            ctx.restore();

            ctx.restore();
        }
    }

    global.CrewmateRenderer = CrewmateRenderer;
})(window);