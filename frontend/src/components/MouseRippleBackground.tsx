import React, { useEffect, useRef } from 'react';

interface Ripple {
    x: number;
    y: number;
    radius: number;
    alpha: number;
    color: string;
}

export const MouseRippleBackground = () => {
    const canvasRef = useRef<HTMLCanvasElement>(null);
    const ripples = useRef<Ripple[]>([]);
    const mouse = useRef({ x: 0, y: 0 });
    const lastMouse = useRef({ x: 0, y: 0 });

    useEffect(() => {
        const canvas = canvasRef.current;
        if (!canvas) return;

        const ctx = canvas.getContext('2d');
        if (!ctx) return;

        let animationFrameId: number;

        const resizeCanvas = () => {
            canvas.width = window.innerWidth;
            canvas.height = window.innerHeight;
        };

        const handleMouseMove = (e: MouseEvent) => {
            mouse.current = { x: e.clientX, y: e.clientY };

            // Only add ripple if mouse moved enough distance to avoid clutter
            const dx = mouse.current.x - lastMouse.current.x;
            const dy = mouse.current.y - lastMouse.current.y;
            const dist = Math.sqrt(dx * dx + dy * dy);

            if (dist > 5) {
                // Determine color based on dark mode preference (simple check)
                const isDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
                const color = isDark ? '255, 255, 255' : '99, 102, 241'; // White or Indigo

                ripples.current.push({
                    x: mouse.current.x,
                    y: mouse.current.y,
                    radius: 0,
                    alpha: 0.5,
                    color: color
                });
                lastMouse.current = { ...mouse.current };
            }
        };

        const animate = () => {
            ctx.clearRect(0, 0, canvas.width, canvas.height);

            // Update and draw ripples
            for (let i = ripples.current.length - 1; i >= 0; i--) {
                const r = ripples.current[i];
                r.radius += 1.5; // Expansion speed
                r.alpha -= 0.01; // Fade speed

                if (r.alpha <= 0) {
                    ripples.current.splice(i, 1);
                } else {
                    ctx.beginPath();
                    ctx.arc(r.x, r.y, r.radius, 0, Math.PI * 2);
                    ctx.fillStyle = `rgba(${r.color}, ${r.alpha})`;
                    ctx.fill();
                }
            }

            animationFrameId = requestAnimationFrame(animate);
        };

        window.addEventListener('resize', resizeCanvas);
        window.addEventListener('mousemove', handleMouseMove);

        resizeCanvas();
        animate();

        return () => {
            window.removeEventListener('resize', resizeCanvas);
            window.removeEventListener('mousemove', handleMouseMove);
            cancelAnimationFrame(animationFrameId);
        };
    }, []);

    return (
        <canvas
            ref={canvasRef}
            className="fixed inset-0 pointer-events-none -z-10"
            style={{ opacity: 0.3 }} // Global opacity to keep it subtle
        />
    );
};
