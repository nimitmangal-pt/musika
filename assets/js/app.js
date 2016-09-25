var socket = new Socket();
socket.on('connection', function() {
    socket.emit('update ui');
});

$(function() {
    $('a[href="#library"]').on('shown.bs.tab', function (e) {
        console.log('amit')
        socket.emit("list media");
    })

    var vol = $('.volume-filler').parent().on('click', function(e) {
        var volume = Math.ceil((100 * e.offsetX)/$(this).width());
        socket.emit('set volume', volume);
    }).end();
    $('.icon-volume-1').parent().click(function() {
        var volume = Math.ceil((vol.width() * 100) / vol.parent().width())
        socket.emit('set volume', Math.max(0, volume - 5));
    });
    $('.icon-volume-2').parent().click(function() {
        var volume = Math.ceil((vol.width() * 100) / vol.parent().width())
        socket.emit('set volume', Math.min(100, volume + 5));
    });
    var play = $('.icon-control-play').parent().click(function() {
        socket.emit('toggle pause');
    }).end();
    $('.icon-control-rewind').parent().click(function() {
        socket.emit('prev');
    }).end();
    $('.icon-control-forward').parent().click(function() {
        socket.emit('next');
    }).end();
    
    // slider.noUiSlider.on('change', function(){
    //     socket.emit('set volume', parseInt(slider.noUiSlider.get()));
    // });

    var shuffle = $('.icon-shuffle')
    var repeat = $('.repeat > span')
    var time = $('.time')

    function timer() {
        setInterval(function() {
            socket.emit('update ui');
        }, 1000);
    };

    timer();

    var queueContainer = $('#nowplaying').find('.row')
    var songEntry = queueContainer.find('.song.hide')

    socket.on("media list", function(data) {
        console.log(data);
        var libraries = riot.mount('library');
        libraries.forEach(function(library){
            library.update({items: data});
        });
    });
    
    socket.on("update ui", function(data) {
        console.log(data);
        vol.css({width: data.volume + '%'});
        // slider.noUiSlider.set(data.volume);
        if(data.random) {
            $('.icon-shuffle').removeClass('text-muted');
        } else {
            $('.icon-shuffle').addClass('text-muted');
        }
        repeat.removeClass('icon-refresh').removeClass('icon-loop').removeClass('text-muted');
        if(data.repeat == 0) {
            repeat.addClass('icon-loop').addClass('text-muted');
        } else if(data.repeat == 1) {
            repeat.addClass('icon-loop');
        } else {
            repeat.addClass('icon-refresh');
        }
        play.removeClass('icon-control-play').removeClass('icon-control-pause');
        if(data.state == "play") {
            play.addClass('icon-control-pause')
        } else {
            play.addClass('icon-control-play')
        }
        if(data.state != "stop") {
            times = data.time.split(':');
            if(parseInt(times[1]) > 3600) {
                time.html(SecondsTohhmmss(times[0], 3) + " / " + SecondsTohhmmss(times[1]));
            } else {
                time.html(SecondsTohhmmss(times[0], 2) + " / " + SecondsTohhmmss(times[1]));
            }
        } else {
            time.html("00:00 / 00:00")
        }
        // queueContainer.find('.song').not('.hide').remove()
        var tags = riot.mount('nowplaying')
        tags.forEach(function(tag) {
            tag.update({items: data.queue})
        });
        
        // if(data.queue) {
        //     if(queueContainer.find('.song').not('.hide').length() > )
        //     for(var i = 0; i < data.queue.length; i++) {
        //         if()
        //     }
        // }
        // data.queue.forEach(function(song) {
        //     songEntry.clone().removeClass('hide').data(song).on('click', function() {
        //         socket.emit("play queued song", song.pos);
        //     }).appendTo(queueContainer).find('img').attr('src', "/coverArt?file=" + song.file).attr('alt', song.Title)
        // });
    })

    var SecondsTohhmmss = function(totalSeconds, precision) {
        var hours   = Math.floor(totalSeconds / 3600);
        var minutes = Math.floor((totalSeconds - (hours * 3600)) / 60);
        var seconds = totalSeconds - (hours * 3600) - (minutes * 60);

        // round seconds
        seconds = Math.round(seconds * 100) / 100
        var result = (seconds  < 10 ? "0" + seconds : seconds);
        if(hours > 0 || (hours == 0 && minutes > 0) || precision > 1) {
            result = (minutes < 10 ? "0" + minutes : minutes) + ":" + result;
        }
        if(hours > 0) {
            result += (hours < 10 ? "0" + hours : hours) + ":" + result;
        }
        return result;
    }

    function Socket(address) {
        this.address = (address || document.location.host);
        var ws;
        var self = this;
        function connect() {
            ws = new WebSocket("ws://" + self.address + "/ws");
        }
        connect();
        var messages = [];
        this.ready = false;
        ws.onopen = function() {
            self.ready = true;
            console.log("open fired " + ws.readyState);
            runHandlersFor('connection');
            if(messages.length > 0) {
                messages.forEach(function(message) {
                    self.emit(message.Action, message.Data);
                });
            }
        }
        ws.onmessage = function(evt) {
            evt = JSON.parse(evt.data);
            if(eventHandlers[evt.Action]) {
                runHandlersFor(evt.Action, evt.Data);
            }
        }
        ws.onerror = function() {
            self.ready = false;
            ws.close()
        }
        ws.onclose = function() {
            self.ready = false;
            runHandlersFor('disconnect');
        }
        this.emit = function(event, data) {
            var message = {Action: event, Data: data};
            if(self.ready) {
                console.log("reported to be ready " + ws.readyState);
                ws.send(JSON.stringify(message));
            } else {
                messages.push(message);
                connect();
            }
        };
        var eventHandlers = []
        this.on = function(eventName, cb) {
            eventHandlers[eventName] = eventHandlers[eventName] || [];
            eventHandlers[eventName].push(cb);
        }
        function runHandlersFor(eventName, data) {
            eventHandlers[eventName].forEach(function(cb) {
                cb(data);
            })
        }
    }
});