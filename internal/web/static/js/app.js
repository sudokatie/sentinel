// Sentinel Dashboard JavaScript

// Auto-refresh dashboard every 30 seconds
(function() {
    if (window.location.pathname === '/') {
        setTimeout(function() {
            window.location.reload();
        }, 30000);
    }
})();

// Simple chart drawing using canvas
function drawResponseChart() {
    var canvas = document.getElementById('responseChart');
    if (!canvas || typeof chartData === 'undefined') return;
    
    var ctx = canvas.getContext('2d');
    var data = chartData.values;
    var labels = chartData.labels;
    var statuses = chartData.statuses;
    
    if (data.length === 0) return;
    
    var width = canvas.width;
    var height = canvas.height;
    var padding = 40;
    var chartWidth = width - padding * 2;
    var chartHeight = height - padding * 2;
    
    // Find max value for scaling
    var maxVal = Math.max.apply(null, data.filter(function(v) { return v > 0; }));
    if (maxVal === 0) maxVal = 100;
    maxVal = maxVal * 1.1; // Add 10% padding
    
    // Clear canvas
    ctx.fillStyle = '#000';
    ctx.fillRect(0, 0, width, height);
    
    // Draw grid
    ctx.strokeStyle = '#222';
    ctx.lineWidth = 1;
    
    // Horizontal grid lines
    for (var i = 0; i <= 4; i++) {
        var y = padding + (chartHeight / 4) * i;
        ctx.beginPath();
        ctx.moveTo(padding, y);
        ctx.lineTo(width - padding, y);
        ctx.stroke();
        
        // Y-axis labels
        var val = Math.round(maxVal - (maxVal / 4) * i);
        ctx.fillStyle = '#666';
        ctx.font = '10px monospace';
        ctx.textAlign = 'right';
        ctx.fillText(val + 'ms', padding - 5, y + 3);
    }
    
    // Draw data
    if (data.length > 1) {
        var stepX = chartWidth / (data.length - 1);
        
        // Draw line
        ctx.beginPath();
        ctx.strokeStyle = '#00ff00';
        ctx.lineWidth = 2;
        
        for (var i = 0; i < data.length; i++) {
            var x = padding + stepX * i;
            var y = padding + chartHeight - (data[i] / maxVal * chartHeight);
            
            if (i === 0) {
                ctx.moveTo(x, y);
            } else {
                ctx.lineTo(x, y);
            }
        }
        ctx.stroke();
        
        // Draw points
        for (var i = 0; i < data.length; i++) {
            var x = padding + stepX * i;
            var y = padding + chartHeight - (data[i] / maxVal * chartHeight);
            
            ctx.beginPath();
            ctx.fillStyle = statuses[i] === 'up' ? '#00ff00' : '#ff4444';
            ctx.arc(x, y, 3, 0, Math.PI * 2);
            ctx.fill();
        }
    }
    
    // Draw axes
    ctx.strokeStyle = '#444';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(padding, padding);
    ctx.lineTo(padding, height - padding);
    ctx.lineTo(width - padding, height - padding);
    ctx.stroke();
}

// Initialize chart on page load
document.addEventListener('DOMContentLoaded', function() {
    drawResponseChart();
});
